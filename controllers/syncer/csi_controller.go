package syncer

import (
	"fmt"

	"github.com/imdario/mergo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/pkg/mergo/transformers"
	"github.com/presslabs/controller-util/pkg/syncer"
	csiv1lpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/util/boolptr"
)

const (
	socketVolumeName                           = "socket-dir"
	ControllerContainerName                    = "ibm-object-csi-controller"
	provisionerContainerName                   = "csi-provisioner"
	controllerLivenessProbeContainerName       = "livenessprobe"
	controllerContainerHealthPortName          = "healthz"
	controllerContainerDefaultHealthPortNumber = 9808
)

type csiControllerSyncer struct {
	driver *crutils.IBMObjectCSI
	obj    runtime.Object
}

// NewCSIControllerSyncer returns a syncer for CSI controller
func NewCSIControllerSyncer(c client.Client, scheme *runtime.Scheme, driver *crutils.IBMObjectCSI) syncer.Interface {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.GetNameForResource(config.CSIController, driver.Name),
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
		},
	}

	sync := &csiControllerSyncer{
		driver: driver,
		obj:    obj,
	}

	return syncer.NewObjectSyncer(config.CSIController.String(), driver.Unwrap(), obj, c, func() error {
		return sync.SyncFn()
	})
}

func (s *csiControllerSyncer) SyncFn() error {
	out := s.obj.(*appsv1.Deployment)

	out.Spec.Selector = metav1.SetAsLabelSelector(s.driver.GetCSIControllerSelectorLabels())
	//out.Spec.ServiceName = config.GetNameForResource(config.CSIController, s.driver.Name)

	controllerLabels := s.driver.GetCSIControllerPodLabels()
	controllerAnnotations := s.driver.GetAnnotations()

	// ensure template
	out.Spec.Template.ObjectMeta.Labels = controllerLabels

	out.ObjectMeta.Labels = controllerLabels
	ensureAnnotations(&out.Spec.Template.ObjectMeta, &out.ObjectMeta, controllerAnnotations)

	err := mergo.Merge(&out.Spec.Template.Spec, s.ensurePodSpec(), mergo.WithTransformers(transformers.PodSpec))
	if err != nil {
		return err
	}

	return nil
}

func (s *csiControllerSyncer) ensurePodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: s.ensureContainersSpec(),
		Volumes:    s.ensureVolumes(),
		//		SecurityContext: &corev1.PodSecurityContext{
		//			FSGroup:   &fsGroup,
		//			RunAsUser: &fsGroup,
		//		},
		Affinity:           s.driver.Spec.Controller.Affinity,
		Tolerations:        s.driver.Spec.Controller.Tolerations,
		ServiceAccountName: config.GetNameForResource(config.CSIControllerServiceAccount, s.driver.Name),
	}
}

func (s *csiControllerSyncer) ensureContainersSpec() []corev1.Container {
	controllerPlugin := s.ensureContainer(ControllerContainerName,
		s.driver.GetCSIControllerImage(),
		[]string{"--endpoint=$(CSI_ENDPOINT)",
			"--servermode=controller",
			"--v=5",
			"--logtostderr=true",
		},
	)

	controllerPlugin.Resources = ensureResources("40m", "800m", "40Mi", "400Mi")

	healthPort := s.driver.Spec.HealthPort
	if healthPort == 0 {
		healthPort = controllerContainerDefaultHealthPortNumber
	}

	controllerPlugin.Ports = ensurePorts(corev1.ContainerPort{
		Name:          controllerContainerHealthPortName,
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
	}
	provisioner := s.ensureContainer(provisionerContainerName,
		s.getCSIProvisionerImage(),
		provisionerArgs,
	)
	provisioner.ImagePullPolicy = s.getCSIProvisionerPullPolicy()
	healthPortArg := fmt.Sprintf("--health-port=%v", healthPort)
	livenessProbe := s.ensureContainer(controllerLivenessProbeContainerName,
		s.getLivenessProbeImage(),
		[]string{
			"--csi-address=/csi/csi.sock",
			healthPortArg,
		},
	)
	livenessProbe.ImagePullPolicy = s.getLivenessProbePullPolicy()

	return []corev1.Container{
		controllerPlugin,
		provisioner,
		livenessProbe,
	}
}

func ensureDefaultResources() corev1.ResourceRequirements {
	return ensureResources("20m", "200m", "20Mi", "200Mi")
}

func ensureResources(cpuRequests, cpuLimits, memoryRequests, memoryLimits string) corev1.ResourceRequirements {
	requests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpuRequests),
		corev1.ResourceMemory: resource.MustParse(memoryRequests),
	}
	limits := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpuLimits),
		corev1.ResourceMemory: resource.MustParse(memoryLimits),
	}

	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}
func (s *csiControllerSyncer) ensureContainer(name, image string, args []string) corev1.Container {
	sc := &corev1.SecurityContext{AllowPrivilegeEscalation: boolptr.False()}
	fillSecurityContextCapabilities(sc)
	return corev1.Container{
		Name:  name,
		Image: image,
		Args:  args,
		//EnvFrom:         s.getEnvSourcesFor(name),
		Env:             s.getEnvFor(name),
		VolumeMounts:    s.getVolumeMountsFor(name),
		SecurityContext: sc,
		Resources:       ensureDefaultResources(),
	}
}

func (s *csiControllerSyncer) envVarFromSecret(sctName, name, key string, opt bool) corev1.EnvVar {
	env := corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: sctName,
				},
				Key:      key,
				Optional: &opt,
			},
		},
	}
	return env
}

func (s *csiControllerSyncer) getEnvFor(name string) []corev1.EnvVar {

	switch name {
	case ControllerContainerName:
		return []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: config.CSIEndpoint,
			},
		}

	case provisionerContainerName:
		return []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: config.ControllerSocketPath,
			},
		}
	}
	return nil
}

func (s *csiControllerSyncer) getVolumeMountsFor(name string) []corev1.VolumeMount {
	switch name {
	case ControllerContainerName, provisionerContainerName:
		return []corev1.VolumeMount{
			{
				Name:      socketVolumeName,
				MountPath: config.ControllerSocketVolumeMountPath,
			},
		}

	case controllerLivenessProbeContainerName:
		return []corev1.VolumeMount{
			{
				Name:      socketVolumeName,
				MountPath: config.ControllerLivenessProbeContainerSocketVolumeMountPath,
			},
		}
	}
	return nil
}

func (s *csiControllerSyncer) ensureVolumes() []corev1.Volume {
	return []corev1.Volume{
		ensureVolume(socketVolumeName, corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}),
	}
}

func (s *csiControllerSyncer) getSidecarByName(name string) *csiv1lpha1.CSISidecar {
	return getSidecarByName(s.driver, name)
}

func (s *csiControllerSyncer) getSidecarImageByName(name string) string {
	sidecar := s.getSidecarByName(name)
	if sidecar != nil {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return s.driver.GetDefaultSidecarImageByName(name)
}

func (s *csiControllerSyncer) getCSIProvisionerImage() string {
	return s.getSidecarImageByName(config.CSIProvisioner)
}

func (s *csiControllerSyncer) getLivenessProbeImage() string {
	return s.getSidecarImageByName(config.LivenessProbe)
}

func (s *csiControllerSyncer) getSidecarPullPolicy(sidecarName string) corev1.PullPolicy {
	sidecar := s.getSidecarByName(sidecarName)
	if sidecar != nil && sidecar.ImagePullPolicy != "" {
		return sidecar.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

func (s *csiControllerSyncer) getCSIProvisionerPullPolicy() corev1.PullPolicy {
	return s.getSidecarPullPolicy(config.CSIProvisioner)
}

func (s *csiControllerSyncer) getLivenessProbePullPolicy() corev1.PullPolicy {
	return s.getSidecarPullPolicy(config.LivenessProbe)
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

func getSidecarByName(driver *crutils.IBMObjectCSI, name string) *csiv1lpha1.CSISidecar {
	for _, sidecar := range driver.Spec.Sidecars {
		if sidecar.Name == name {
			return &sidecar
		}
	}
	return nil
}
