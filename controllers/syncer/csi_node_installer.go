// Package syncer ..
package syncer

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
)

// NewCSIBinsInstallerDaemonSet returns the installer DaemonSet for kube-system.
// Managed directly by the controller (not via presslabs syncer) because
// cross-namespace ownerReferences are not allowed in Kubernetes.
func NewCSIBinsInstallerDaemonSet(driver *crutils.IBMObjectCSI) *appsv1.DaemonSet {
	installerLabels := driver.GetCSIBinsInstallerPodLabels()
	selectorLabels := driver.GetCSIBinsInstallerSelectorLabels()

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.CSIInstallerName,
			Namespace: constants.CSIInstallerNamespace,
			Labels:    installerLabels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: metav1.SetAsLabelSelector(selectorLabels),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: installerLabels,
				},
				Spec: ensureBinsInstallerPodSpec(driver),
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: func(i intstr.IntOrString) *intstr.IntOrString { return &i }(intstr.FromInt(1)),
				},
			},
		},
	}

	return ds
}

func ensureBinsInstallerPodSpec(driver *crutils.IBMObjectCSI) corev1.PodSpec {
	privileged := true
	runAsUser := int64(0)

	hostPathType := corev1.HostPathType("")

	spec := corev1.PodSpec{
		HostNetwork:                   true,
		HostPID:                       true,
		PriorityClassName:             constants.CSIInstallerPriorityClassName,
		TerminationGracePeriodSeconds: func(i int64) *int64 { return &i }(30),
		InitContainers: []corev1.Container{
			{
				Name:  constants.CSIInstallerContainer,
				Image: driver.GetCSIBinsInstallerImage(),
				Command: []string{
					"/bin/bash",
					"-c",
					"/home/cos-mounters/cos-csi-installer/cos-csi-install.sh && /home/cos-mounters/copy-cos-mounter-bins.sh",
				},
				ImagePullPolicy: getBinsInstallerImagePullPolicy(driver),
				Resources:       getBinsInstallerResourceRequests(driver),
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
					RunAsUser:  &runAsUser,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "host-root",
						MountPath: "/host",
					},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:            "pause",
				Image:           constants.CSIInstallerPauseImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "host-root",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/",
						Type: &hostPathType,
					},
				},
			},
		},
		Tolerations: getBinsInstallerTolerations(driver),
		Affinity:    buildBinsInstallerAffinity(driver),
	}

	return spec
}

// buildBinsInstallerAffinity mirrors buildNodeAffinity in csi_node.go.
// When restrictNodeServerScheduling is true it injects the cos.csi.ibm.io/csi-node=true
// label requirement so the installer only runs on nodes that also run the node plugin.
func buildBinsInstallerAffinity(driver *crutils.IBMObjectCSI) *corev1.Affinity {
	if driver.Spec.BinsInstaller == nil {
		return nil
	}

	affinity := driver.Spec.BinsInstaller.Affinity
	restrictScheduling := driver.Spec.BinsInstaller.RestrictNodeServerScheduling

	if restrictScheduling == "true" {
		if affinity != nil && affinity.NodeAffinity != nil &&
			affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {

			for i := range affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				term := &affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[i]

				hasRequirement := false
				for _, expr := range term.MatchExpressions {
					if expr.Key == constants.CSIAddonNodeLabelKey {
						hasRequirement = true
						break
					}
				}

				if !hasRequirement {
					term.MatchExpressions = append(term.MatchExpressions, corev1.NodeSelectorRequirement{
						Key:      constants.CSIAddonNodeLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{constants.CSIAddonNodeLabelValue},
					})
				}
			}
		}
	}

	return affinity
}

func getBinsInstallerTolerations(driver *crutils.IBMObjectCSI) []corev1.Toleration {
	if driver.Spec.BinsInstaller != nil && len(driver.Spec.BinsInstaller.Tolerations) > 0 {
		return driver.Spec.BinsInstaller.Tolerations
	}
	// default: tolerate everything — installer must run on all nodes
	return []corev1.Toleration{{Operator: corev1.TolerationOpExists}}
}

func getBinsInstallerImagePullPolicy(driver *crutils.IBMObjectCSI) corev1.PullPolicy {
	if driver.Spec.BinsInstaller != nil && driver.Spec.BinsInstaller.ImagePullPolicy != "" {
		return driver.Spec.BinsInstaller.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

func getBinsInstallerResourceRequests(driver *crutils.IBMObjectCSI) corev1.ResourceRequirements {
	resources := driver.GetCSIBinsInstallerResourceRequests()

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
