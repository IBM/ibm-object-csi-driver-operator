// Package syncer ..
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

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	"github.com/presslabs/controller-util/pkg/mergo/transformers"
	"github.com/presslabs/controller-util/pkg/syncer"
)

type csiNodeSyncer struct {
	driver *crutils.IBMObjectCSI
	obj    runtime.Object
}

// NewCSINodeSyncer returns a syncer for CSI node
func NewCSINodeSyncer(c client.Client, driver *crutils.IBMObjectCSI) syncer.Interface {
	obj := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.GetResourceName(constants.CSINode),
			Namespace:   driver.Namespace,
			Annotations: driver.GetAnnotations(),
			Labels:      driver.GetLabels(),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: metav1.SetAsLabelSelector(driver.GetCSINodeSelectorLabels()),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      driver.GetCSINodePodLabels(),
					Annotations: driver.GetAnnotations(),
				},
				Spec: corev1.PodSpec{},
			},
			MinReadySeconds: 30,
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: func(i intstr.IntOrString) *intstr.IntOrString { return &i }(intstr.FromString("10%")),
				},
			},
		},
	}

	sync := &csiNodeSyncer{
		driver: driver,
		obj:    obj,
	}

	return syncer.NewObjectSyncer(constants.CSINode, driver.Unwrap(), obj, c, func() error {
		return sync.SyncFn()
	})
}

func (s *csiNodeSyncer) SyncFn() error {
	out := s.obj.(*appsv1.DaemonSet)

	// Fetch initContainers from NodeSpec in the CR
	crInitContainers := s.driver.Spec.Node.InitContainers

	// Update DaemonSet initContainers if necessary
	if len(crInitContainers) > 0 {
		// Set initContainers to the DaemonSet
		out.Spec.Template.Spec.InitContainers = crInitContainers
	} else {
		// If no initContainers defined in NodeSpec, clear DaemonSet initContainers
		out.Spec.Template.Spec.InitContainers = nil
	}

	out.Spec.Selector = metav1.SetAsLabelSelector(s.driver.GetCSINodeSelectorLabels())

	nodeLabels := s.driver.GetCSINodePodLabels()

	// ensure template
	out.Spec.Template.ObjectMeta.Labels = nodeLabels
	nodeAnnotations := s.driver.GetAnnotations()

	out.ObjectMeta.Labels = nodeLabels
	ensureAnnotations(&out.Spec.Template.ObjectMeta, &out.ObjectMeta, nodeAnnotations)

	err := mergo.Merge(&out.Spec.Template.Spec, s.ensurePodSpec(), mergo.WithTransformers(transformers.PodSpec))
	if err != nil {
		return err
	}

	return nil
}

func (s *csiNodeSyncer) ensurePodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		InitContainers: s.ensureInitContainers(),
		Containers:     s.ensureContainersSpec(),
		Volumes:        s.ensureVolumes(),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: util.True(),
			RunAsUser:    func(uid int64) *int64 { return &uid }(0),
			RunAsGroup:   func(uid int64) *int64 { return &uid }(0),
		},
		Affinity:           s.driver.Spec.Node.Affinity,
		Tolerations:        s.driver.Spec.Node.Tolerations,
		ServiceAccountName: constants.GetResourceName(constants.CSINodeServiceAccount),
		PriorityClassName:  constants.CSINodePriorityClassName,
		// HostIPC:            true,
		// HostNetwork:        true,
		// HostPID:            true,
	}
}

func (s *csiNodeSyncer) ensureInitContainers() []corev1.Container {
	// initContainer
	initContainer := s.ensureContainer("cos-installer",
		"bhagyak1/install-cos-driver:m14v01",
		[]string{
			"/bin/sh",
			"-c",
			"/cos-installer/installS3fsDeps.sh; /cos-installer/installS3fs.sh; /cos-installer/installRclone.sh;  sleep 300",
		},
	)

	initContainer.ImagePullPolicy = s.driver.Spec.Node.ImagePullPolicy

	initContainer.SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot: util.False(),
		Privileged:   util.True(),
		RunAsUser:    func(uid int64) *int64 { return &uid }(0),
	}
	fillSecurityContextCapabilities(
		initContainer.SecurityContext,
	)

	initContainer.TTY = *util.True()
	initContainer.VolumeMounts = []corev1.VolumeMount{
		// {
		// 	Name:      "host-root",
		// 	MountPath: "/host",
		// },
		{
			Name:      "usr-local",
			MountPath: "/host/local",
		},
	}

	return []corev1.Container{initContainer}
}

func (s *csiNodeSyncer) ensureContainersSpec() []corev1.Container {
	// node plugin container
	nodePlugin := s.ensureContainer(constants.NodeContainerName,
		s.driver.GetCSINodeImage(),
		[]string{
			"--servermode=node",
			"--endpoint=$(CSI_ENDPOINT)",
			"--nodeid=$(KUBE_NODE_NAME)",
			"--logtostderr=true",
			"--v=5",
		},
	)

	nodePlugin.Resources = getCSINodeResourceRequests(s.driver)

	healthPort := s.driver.Spec.HealthPort
	if healthPort == 0 {
		healthPort = constants.HealthPortNumber
	}
	nodePlugin.Ports = ensurePorts(corev1.ContainerPort{
		Name:          constants.HealthPortName,
		ContainerPort: int32(healthPort),
	})

	nodePlugin.ImagePullPolicy = s.driver.Spec.Node.ImagePullPolicy

	nodeContainerHealthPort := intstr.FromInt(int(healthPort))
	nodePlugin.LivenessProbe = ensureProbe(10, 3, 10, corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   "/healthz",
			Port:   nodeContainerHealthPort,
			Scheme: corev1.URISchemeHTTP,
		},
	})

	nodePlugin.SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot: util.False(),
		Privileged:   util.True(),
		RunAsUser:    func(uid int64) *int64 { return &uid }(0),
	}
	fillSecurityContextCapabilities(
		nodePlugin.SecurityContext,
	)

	// node driver registrar sidecar
	registrar := s.ensureContainer(constants.CSINodeDriverRegistrar,
		s.getCSINodeDriverRegistrarImage(),
		[]string{
			"--csi-address=$(ADDRESS)",
			"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
			"--v=5",
		},
	)
	registrar.SecurityContext = &corev1.SecurityContext{RunAsNonRoot: util.False(),
		RunAsUser:  func(uid int64) *int64 { return &uid }(0),
		Privileged: util.False(),
	}
	fillSecurityContextCapabilities(registrar.SecurityContext)
	registrar.ImagePullPolicy = s.getCSINodeDriverRegistrarPullPolicy()
	registrar.Resources = getSidecarResourceRequests(s.driver, constants.CSINodeDriverRegistrar)

	// liveness probe sidecar
	healthPortArg := fmt.Sprintf("--health-port=%v", healthPort)
	livenessProbe := s.ensureContainer(constants.LivenessProbe,
		s.getLivenessProbeImage(),
		[]string{
			"--csi-address=/csi/csi.sock",
			healthPortArg,
		},
	)
	livenessProbe.SecurityContext = &corev1.SecurityContext{RunAsNonRoot: util.False(),
		RunAsUser:  func(uid int64) *int64 { return &uid }(0),
		Privileged: util.False(),
	}
	fillSecurityContextCapabilities(livenessProbe.SecurityContext)
	livenessProbe.ImagePullPolicy = s.getCSINodeDriverRegistrarPullPolicy()
	livenessProbe.Resources = getSidecarResourceRequests(s.driver, constants.LivenessProbe)

	return []corev1.Container{
		nodePlugin,
		registrar,
		livenessProbe,
	}
}

func (s *csiNodeSyncer) ensureContainer(name, image string, args []string) corev1.Container {
	return corev1.Container{
		Name:         name,
		Image:        image,
		Args:         args,
		Env:          s.getEnvFor(name),
		VolumeMounts: s.getVolumeMountsFor(name),
	}
}

func envVarFromField(name, fieldPath string) corev1.EnvVar {
	env := corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: constants.APIVersion,
				FieldPath:  fieldPath,
			},
		},
	}
	return env
}

func (s *csiNodeSyncer) getEnvFor(name string) []corev1.EnvVar {
	switch name {
	case constants.NodeContainerName:
		return []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: constants.CSINodeEndpoint,
			},
			envVarFromField("KUBE_NODE_NAME", "spec.nodeName"),
		}

	case constants.CSINodeDriverRegistrar:
		return []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: constants.NodeSocketPath,
			},
			{
				Name:  "DRIVER_REG_SOCK_PATH",
				Value: constants.NodeRegistrarSocketPath,
			},
		}
	}
	return nil
}

func (s *csiNodeSyncer) getVolumeMountsFor(name string) []corev1.VolumeMount {
	mountPropagationB := corev1.MountPropagationBidirectional

	switch name {
	case constants.NodeContainerName:
		return []corev1.VolumeMount{
			{
				Name:      constants.PluginVolumeName,
				MountPath: constants.NodeSocketVolumeMountPath,
			},
			{
				Name:             "kubelet-dir",
				MountPath:        "/var/lib/kubelet",
				MountPropagation: &mountPropagationB,
			},
			{
				Name:             "kubelet-dir-ibm",
				MountPath:        "/var/data/kubelet",
				MountPropagation: &mountPropagationB,
			},
			{
				Name:      "fuse-device",
				MountPath: "/dev/fuse",
			},
			{
				Name:      "log-dev",
				MountPath: "/dev/log",
			},
			{
				Name:      "host-log",
				MountPath: "/host/var/log",
			},
			{
				Name:      "coscsi-socket",
				MountPath: "/var/lib/coscsi.sock",
				ReadOnly:  false,
			},
			{
				Name:      "mount-path",
				MountPath: "/var/lib/cos-csi",
			},
		}

	case constants.CSINodeDriverRegistrar:
		return []corev1.VolumeMount{
			{
				Name:      constants.PluginVolumeName,
				MountPath: constants.NodeSocketVolumeMountPath,
			},
			{
				Name:      constants.RegistrationVolumeName,
				MountPath: constants.RegistrationVolumeMountPath,
			},
		}

	case constants.LivenessProbe:
		return []corev1.VolumeMount{
			{
				Name:      constants.PluginVolumeName,
				MountPath: constants.NodeSocketVolumeMountPath,
			},
		}
	}
	return nil
}

func (s *csiNodeSyncer) ensureVolumes() []corev1.Volume {
	return []corev1.Volume{
		ensureVolume("kubelet-dir", ensureHostPathVolumeSource("/var/lib/kubelet", "Directory")),
		ensureVolume("plugin-dir", ensureHostPathVolumeSource("/var/lib/kubelet/plugins/cos.s3.csi.ibm.io", "DirectoryOrCreate")),
		ensureVolume("registration-dir", ensureHostPathVolumeSource("/var/lib/kubelet/plugins_registry", "Directory")),
		ensureVolume("kubelet-dir-ibm", ensureHostPathVolumeSource("/var/data/kubelet", "DirectoryOrCreate")),
		ensureVolume("fuse-device", ensureHostPathVolumeSource("/dev/fuse", "")),
		ensureVolume("log-dev", ensureHostPathVolumeSource("/dev/log", "")),
		ensureVolume("host-log", ensureHostPathVolumeSource("/var/log", "")),
		//ensureVolume("host-root", ensureHostPathVolumeSource("/", "")),
		ensureVolume("usr-local", ensureHostPathVolumeSource("/usr/local", "")),
		ensureVolume("coscsi-socket", ensureHostPathVolumeSource("/var/lib/coscsi.sock", "Socket")),
		ensureVolume("mount-path", ensureHostPathVolumeSource("/var/lib/cos-csi", "DirectoryOrCreate")),
	}
}

func (s *csiNodeSyncer) getSidecarByName(name string) *objectdriverv1alpha1.CSISidecar {
	return getSidecarByName(s.driver, name)
}

func (s *csiNodeSyncer) getCSINodeDriverRegistrarImage() string {
	sidecar := s.getSidecarByName(constants.CSINodeDriverRegistrar)
	if sidecar != nil {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return ""
}

func (s *csiNodeSyncer) getLivenessProbeImage() string {
	sidecar := s.getSidecarByName(constants.LivenessProbe)
	if sidecar != nil {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return ""
}

func (s *csiNodeSyncer) getCSINodeDriverRegistrarPullPolicy() corev1.PullPolicy {
	sidecar := s.getSidecarByName(constants.CSINodeDriverRegistrar)
	if sidecar != nil && sidecar.ImagePullPolicy != "" {
		return sidecar.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

func ensureHostPathVolumeSource(path, pathType string) corev1.VolumeSource {
	t := corev1.HostPathType(pathType)

	return corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: path,
			Type: &t,
		},
	}
}

func fillSecurityContextCapabilities(sc *corev1.SecurityContext, add ...string) {
	sc.Capabilities = &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	}

	if len(add) > 0 {
		adds := []corev1.Capability{}
		for _, a := range add {
			adds = append(adds, corev1.Capability(a))
		}
		sc.Capabilities.Add = adds
	}
}

func getCSINodeResourceRequests(driver *crutils.IBMObjectCSI) corev1.ResourceRequirements {
	resources := driver.GetCSINodeResourceRequests()

	var requests, limits corev1.ResourceList

	if resources.Requests.CPU != "" && resources.Requests.Memory != "" {
		requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(resources.Requests.CPU),
			corev1.ResourceMemory: resource.MustParse(resources.Requests.Memory),
		}
	}
	if resources.Limits.CPU != "" && resources.Limits.Memory != "" {
		limits = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(resources.Limits.CPU),
			corev1.ResourceMemory: resource.MustParse(resources.Limits.Memory),
		}
	}

	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}
