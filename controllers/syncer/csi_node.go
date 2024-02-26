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

	"github.com/presslabs/controller-util/pkg/mergo/transformers"
	"github.com/presslabs/controller-util/pkg/syncer"
	csiv1alpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/util/boolptr"
)

const (
	registrationVolumeName               = "registration-dir"
	NodeContainerName                    = "ibm-object-csi-node"
	csiNodeDriverRegistrarContainerName  = "csi-node-driver-registrar"
	nodeLivenessProbeContainerName       = "livenessprobe"
	pluginVolumeName                     = "plugin-dir"
	nodeContainerHealthPortName          = "healthz"
	nodeContainerDefaultHealthPortNumber = 9808

	registrationVolumeMountPath = "/registration"
)

type csiNodeSyncer struct {
	driver *crutils.IBMObjectCSI
	obj    runtime.Object
}

// NewCSINodeSyncer returns a syncer for CSI node
func NewCSINodeSyncer(c client.Client, scheme *runtime.Scheme, driver *crutils.IBMObjectCSI) syncer.Interface {
	obj := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.GetNameForResource(config.CSINode, driver.Name),
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
		},
	}

	sync := &csiNodeSyncer{
		driver: driver,
		obj:    obj,
	}

	return syncer.NewObjectSyncer(config.CSINode.String(), driver.Unwrap(), obj, c, func() error {
		return sync.SyncFn()
	})
}

func (s *csiNodeSyncer) SyncFn() error {
	out := s.obj.(*appsv1.DaemonSet)

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
		Containers:         s.ensureContainersSpec(),
		Volumes:            s.ensureVolumes(),
		ServiceAccountName: config.GetNameForResource(config.CSINodeServiceAccount, s.driver.Name),
	}
}

func (s *csiNodeSyncer) ensureContainersSpec() []corev1.Container {
	// node plugin container
	nodePlugin := s.ensureContainer(NodeContainerName,
		s.driver.GetCSINodeImage(),
		[]string{
			"--servermode=node",
			"--endpoint=$(CSI_ENDPOINT)",
			"--nodeid=$(KUBE_NODE_NAME)",
			"--logtostderr=true",
			"--v=5",
		},
	)

	nodePlugin.Resources = ensureResources("40m", "1000m", "40Mi", "400Mi")

	healthPort := s.driver.Spec.HealthPort
	if healthPort == 0 {
		healthPort = nodeContainerDefaultHealthPortNumber
	}
	nodePlugin.Ports = ensurePorts(corev1.ContainerPort{
		Name:          nodeContainerHealthPortName,
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
		Privileged:               boolptr.True(),
		AllowPrivilegeEscalation: boolptr.True(),
	}
	fillSecurityContextCapabilities(
		nodePlugin.SecurityContext,
		"SYS_ADMIN",
	)

	// node driver registrar sidecar
	registrar := s.ensureContainer(csiNodeDriverRegistrarContainerName,
		s.getCSINodeDriverRegistrarImage(),
		[]string{
			"--csi-address=$(ADDRESS)",
			"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
			"--v=5",
		},
	)
	registrar.SecurityContext = &corev1.SecurityContext{AllowPrivilegeEscalation: boolptr.False()}
	fillSecurityContextCapabilities(registrar.SecurityContext)
	registrar.ImagePullPolicy = s.getCSINodeDriverRegistrarPullPolicy()

	// liveness probe sidecar
	healthPortArg := fmt.Sprintf("--health-port=%v", healthPort)
	livenessProbe := s.ensureContainer(nodeLivenessProbeContainerName,
		s.getLivenessProbeImage(),
		[]string{
			"--csi-address=/csi/csi.sock",
			healthPortArg,
		},
	)
	livenessProbe.SecurityContext = &corev1.SecurityContext{AllowPrivilegeEscalation: boolptr.False()}
	fillSecurityContextCapabilities(livenessProbe.SecurityContext)
	livenessProbe.ImagePullPolicy = s.getCSINodeDriverRegistrarPullPolicy()

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
		Resources:    ensureDefaultResources(),
	}
}

func envVarFromField(name, fieldPath string) corev1.EnvVar {
	env := corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: config.APIVersion,
				FieldPath:  fieldPath,
			},
		},
	}
	return env
}

func (s *csiNodeSyncer) getEnvFor(name string) []corev1.EnvVar {

	switch name {
	case NodeContainerName:
		return []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: config.CSINodeEndpoint,
			},
			envVarFromField("KUBE_NODE_NAME", "spec.nodeName"),
		}

	case csiNodeDriverRegistrarContainerName:
		return []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: config.NodeSocketPath,
			},
			{
				Name:  "DRIVER_REG_SOCK_PATH",
				Value: config.NodeRegistrarSocketPath,
			},
		}
	}
	return nil
}

func (s *csiNodeSyncer) getVolumeMountsFor(name string) []corev1.VolumeMount {
	mountPropagationB := corev1.MountPropagationBidirectional

	switch name {
	case NodeContainerName:
		return []corev1.VolumeMount{
			{
				Name:      pluginVolumeName,
				MountPath: config.NodeSocketVolumeMountPath,
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
		}

	case csiNodeDriverRegistrarContainerName:
		return []corev1.VolumeMount{
			{
				Name:      pluginVolumeName,
				MountPath: config.NodeSocketVolumeMountPath,
			},
			{
				Name:      registrationVolumeName,
				MountPath: registrationVolumeMountPath,
			},
		}

	case nodeLivenessProbeContainerName:
		return []corev1.VolumeMount{
			{
				Name:      pluginVolumeName,
				MountPath: config.NodeSocketVolumeMountPath,
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
	}
}

func (s *csiNodeSyncer) getSidecarByName(name string) *csiv1alpha1.CSISidecar {
	return getSidecarByName(s.driver, name)
}

func (s *csiNodeSyncer) getCSINodeDriverRegistrarImage() string {
	sidecar := s.getSidecarByName(config.CSINodeDriverRegistrar)
	if sidecar != nil {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return s.driver.GetDefaultSidecarImageByName(config.CSINodeDriverRegistrar)
}

func (s *csiNodeSyncer) getLivenessProbeImage() string {
	sidecar := s.getSidecarByName(config.LivenessProbe)
	if sidecar != nil {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return s.driver.GetDefaultSidecarImageByName(config.LivenessProbe)
}

func (s *csiNodeSyncer) getCSINodeDriverRegistrarPullPolicy() corev1.PullPolicy {
	sidecar := s.getSidecarByName(config.CSINodeDriverRegistrar)
	if sidecar != nil && sidecar.ImagePullPolicy != "" {
		return sidecar.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

func (s *csiNodeSyncer) getLivenessProbePullPolicy() corev1.PullPolicy {
	sidecar := s.getSidecarByName(config.LivenessProbe)
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
