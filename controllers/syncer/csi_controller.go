// Package syncer ...
package syncer

import (
	"fmt"

	"github.com/imdario/mergo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	"github.com/presslabs/controller-util/pkg/mergo/transformers"
	"github.com/presslabs/controller-util/pkg/syncer"
)

type csiControllerSyncer struct {
	driver       *crutils.IBMObjectCSI
	obj          runtime.Object
	iaaSProvider string
}

// NewCSIControllerSyncer returns a syncer for CSI controller
func NewCSIControllerSyncer(c client.Client, driver *crutils.IBMObjectCSI, iaaSProvider string) syncer.Interface {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.GetResourceName(constants.CSIController),
			Namespace:   driver.Namespace,
			Annotations: driver.GetAnnotations(),
			Labels:      driver.GetLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: metav1.SetAsLabelSelector(driver.GetCSIControllerSelectorLabels()),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      driver.GetCSIControllerPodLabels(),
					Annotations: driver.GetAnnotations(),
				},
				Spec: corev1.PodSpec{},
			},
			MinReadySeconds: 30,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}

	sync := &csiControllerSyncer{
		driver:       driver,
		obj:          obj,
		iaaSProvider: iaaSProvider,
	}

	return syncer.NewObjectSyncer(constants.CSIController, driver.Unwrap(), obj, c, func() error {
		return sync.SyncFn()
	})
}

func (s *csiControllerSyncer) SyncFn() error {
	out := s.obj.(*appsv1.Deployment)

	out.Spec.Selector = metav1.SetAsLabelSelector(s.driver.GetCSIControllerSelectorLabels())

	controllerLabels := s.driver.GetCSIControllerPodLabels()
	controllerAnnotations := s.driver.GetAnnotations()

	// ensure template
	out.Spec.Template.Labels = controllerLabels

	out.Labels = controllerLabels
	ensureAnnotations(&out.Spec.Template.ObjectMeta, &out.ObjectMeta, controllerAnnotations)

	podSpec, err := s.ensurePodSpec()
	if err != nil {
		return err
	}

	err = mergo.Merge(&out.Spec.Template.Spec, podSpec, mergo.WithTransformers(transformers.PodSpec))
	if err != nil {
		return err
	}

	return nil
}

func (s *csiControllerSyncer) ensurePodSpec() (corev1.PodSpec, error) {
	containers, err := s.ensureContainersSpec()
	if err != nil {
		return corev1.PodSpec{}, err
	}
	return corev1.PodSpec{
		Containers: containers,
		Volumes:    s.ensureVolumes(),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: util.True(),
			RunAsUser:    func(uid int64) *int64 { return &uid }(2121),
			RunAsGroup:   func(uid int64) *int64 { return &uid }(2121),
		},
		Affinity:           s.driver.Spec.Controller.Affinity,
		Tolerations:        s.driver.Spec.Controller.Tolerations,
		ServiceAccountName: constants.GetResourceName(constants.CSIControllerServiceAccount),
		PriorityClassName:  constants.CSIControllerPriorityClassName,
	}, nil
}

func (s *csiControllerSyncer) ensureContainersSpec() ([]corev1.Container, error) {
	controllerPlugin := s.ensureContainer(constants.ControllerContainerName,
		s.driver.GetCSIControllerImage(),
		[]string{"--endpoint=$(CSI_ENDPOINT)",
			"--servermode=controller",
			"--v=5",
			"--logtostderr=true",
		},
	)

	var err error
	controllerPlugin.Resources, err = getResourceRequirements(s.driver.Spec.Controller.Resources)
	if err != nil {
		return nil, fmt.Errorf("container %s: %w", constants.ControllerContainerName, err)
	}

	healthPort := s.driver.Spec.HealthPort
	if healthPort == 0 {
		healthPort = constants.HealthPortNumber
	}

	controllerPlugin.Ports = ensurePorts(corev1.ContainerPort{
		Name:          constants.HealthPortName,
		ContainerPort: int32(healthPort),
	})
	controllerPlugin.ImagePullPolicy = s.driver.Spec.Controller.ImagePullPolicy

	controllerContainerHealthPort := intstr.FromInt(int(healthPort))
	controllerPlugin.LivenessProbe = ensureProbe(10, 100, 5, corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   "/healthz",
			Port:   controllerContainerHealthPort,
			Scheme: corev1.URISchemeHTTP,
		},
	})

	provisionerArgs := []string{
		"--csi-address=$(ADDRESS)",
		"--v=5",
		"--timeout=120s",
		"--extra-create-metadata=true",
	}
	provisioner := s.ensureContainer(constants.CSIProvisioner,
		s.getCSIProvisionerImage(),
		provisionerArgs,
	)
	provisioner.ImagePullPolicy = s.getCSIProvisionerPullPolicy()
	provisioner.Resources, err = getSidecarResourceRequests(s.driver, constants.CSIProvisioner)
	if err != nil {
		return nil, err
	}

	healthPortArg := fmt.Sprintf("--health-port=%v", healthPort)
	livenessProbe := s.ensureContainer(constants.LivenessProbe,
		s.getLivenessProbeImage(),
		[]string{
			"--csi-address=/csi/csi.sock",
			healthPortArg,
		},
	)
	livenessProbe.ImagePullPolicy = s.getLivenessProbePullPolicy()
	livenessProbe.Resources, err = getSidecarResourceRequests(s.driver, constants.LivenessProbe)
	if err != nil {
		return nil, err
	}

	return []corev1.Container{
		controllerPlugin,
		provisioner,
		livenessProbe,
	}, nil
}

func (s *csiControllerSyncer) ensureContainer(name, image string, args []string) corev1.Container {
	sc := &corev1.SecurityContext{
		AllowPrivilegeEscalation: util.False(),
	}
	fillSecurityContextCapabilities(sc)
	return corev1.Container{
		Name:            name,
		Image:           image,
		Args:            args,
		Env:             s.getEnvFor(name),
		VolumeMounts:    s.getVolumeMountsFor(name),
		SecurityContext: sc,
	}
}

func (s *csiControllerSyncer) getEnvFor(name string) []corev1.EnvVar {
	switch name {
	case constants.ControllerContainerName:
		envVars := []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: constants.CSIEndpoint,
			},
		}
		envVars = append(envVars, getEndpointEnvVars(s.iaaSProvider)...)
		return envVars

	case constants.CSIProvisioner:
		return []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: constants.ControllerSocketPath,
			},
		}
	}
	return nil
}

func (s *csiControllerSyncer) getVolumeMountsFor(name string) []corev1.VolumeMount {
	switch name {
	case constants.ControllerContainerName, constants.CSIProvisioner:
		return []corev1.VolumeMount{
			{
				Name:      constants.SocketVolumeName,
				MountPath: constants.ControllerSocketVolumeMountPath,
			},
		}

	case constants.LivenessProbe:
		return []corev1.VolumeMount{
			{
				Name:      constants.SocketVolumeName,
				MountPath: constants.ControllerLivenessProbeContainerSocketVolumeMountPath,
			},
		}
	}
	return nil
}

func (s *csiControllerSyncer) ensureVolumes() []corev1.Volume {
	return []corev1.Volume{
		ensureVolume(constants.SocketVolumeName, corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}),
	}
}

func (s *csiControllerSyncer) getSidecarByName(name string) *objectdriverv1alpha1.CSISidecar {
	return getSidecarByName(s.driver, name)
}

func (s *csiControllerSyncer) getSidecarImageByName(name string) string {
	sidecar := s.getSidecarByName(name)
	if sidecar != nil {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return ""
}

func (s *csiControllerSyncer) getCSIProvisionerImage() string {
	return s.getSidecarImageByName(constants.CSIProvisioner)
}

func (s *csiControllerSyncer) getLivenessProbeImage() string {
	return s.getSidecarImageByName(constants.LivenessProbe)
}

func (s *csiControllerSyncer) getSidecarPullPolicy(sidecarName string) corev1.PullPolicy {
	sidecar := s.getSidecarByName(sidecarName)
	if sidecar != nil && sidecar.ImagePullPolicy != "" {
		return sidecar.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

func (s *csiControllerSyncer) getCSIProvisionerPullPolicy() corev1.PullPolicy {
	return s.getSidecarPullPolicy(constants.CSIProvisioner)
}

func (s *csiControllerSyncer) getLivenessProbePullPolicy() corev1.PullPolicy {
	return s.getSidecarPullPolicy(constants.LivenessProbe)
}

func ensurePorts(ports ...corev1.ContainerPort) []corev1.ContainerPort {
	return ports
}

func ensureProbe(delay, timeout, period int32, handler corev1.ProbeHandler) *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: delay,
		TimeoutSeconds:      timeout,
		PeriodSeconds:       period,
		ProbeHandler:        handler,
		SuccessThreshold:    1,
		FailureThreshold:    30,
	}
}

func ensureVolume(name string, source corev1.VolumeSource) corev1.Volume {
	return corev1.Volume{
		Name:         name,
		VolumeSource: source,
	}
}

func getSidecarByName(driver *crutils.IBMObjectCSI, name string) *objectdriverv1alpha1.CSISidecar {
	for _, sidecar := range driver.Spec.Sidecars {
		if sidecar.Name == name {
			return &sidecar
		}
	}
	return nil
}

func getSidecarResourceRequests(driver *crutils.IBMObjectCSI, sidecarName string) (corev1.ResourceRequirements, error) {
	sidecar := getSidecarByName(driver, sidecarName)
	if sidecar == nil {
		return corev1.ResourceRequirements{}, nil
	}
	res, err := getResourceRequirements(sidecar.Resources)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("sidecar %s: %w", sidecarName, err)
	}
	return res, nil
}
