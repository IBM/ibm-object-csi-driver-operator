// Package crutils ...
package crutils

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
)

// GenerateCSIDriver ...
func (c *IBMObjectCSI) GenerateCSIDriver() *storagev1.CSIDriver {
	defaultFSGroupPolicy := storagev1.FileFSGroupPolicy
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.DriverName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       constants.ObjectCSIDriver,
				"app.kubernetes.io/part-of":    constants.CSIDriverName,
				"app.kubernetes.io/managed-by": constants.CSIOperatorName,
			},
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: util.False(),
			PodInfoOnMount: util.True(),
			FSGroupPolicy:  &defaultFSGroupPolicy,
		},
	}
}

// GenerateControllerServiceAccount ...
func (c *IBMObjectCSI) GenerateControllerServiceAccount() *corev1.ServiceAccount {
	return getServiceAccount(c, constants.CSIControllerServiceAccount)
}

// GenerateNodeServiceAccount ...
func (c *IBMObjectCSI) GenerateNodeServiceAccount() *corev1.ServiceAccount {
	return getServiceAccount(c, constants.CSINodeServiceAccount)
}

func getServiceAccount(c *IBMObjectCSI, serviceAccountResourceName string) *corev1.ServiceAccount {
	secrets := GetImagePullSecrets(c.Spec.ImagePullSecrets)
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GetResourceName(serviceAccountResourceName),
			Namespace: c.Namespace,
			Labels:    c.GetLabels(),
		},
		ImagePullSecrets: secrets,
	}
}

// GenerateExternalProvisionerClusterRole ...
func (c *IBMObjectCSI) GenerateExternalProvisionerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.GetResourceName(constants.ExternalProvisionerClusterRole),
			Labels: constants.CommonCSIResourceLabels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{constants.SecretsResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.PersistentVolumesResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList, constants.VerbWatch, constants.VerbPatch, constants.VerbCreate, constants.VerbDelete},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.PersistentVolumeClaimsResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList, constants.VerbWatch, constants.VerbUpdate},
			},
			{
				APIGroups: []string{constants.StorageAPIGroup},
				Resources: []string{constants.StorageClassesResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList, constants.VerbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.EventsResource},
				Verbs:     []string{constants.VerbList, constants.VerbWatch, constants.VerbCreate, constants.VerbUpdate, constants.VerbPatch},
			},
			{
				APIGroups: []string{constants.StorageAPIGroup},
				Resources: []string{constants.CSINodesResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList, constants.VerbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.NodesResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList, constants.VerbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.ConfigMapResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList},
			},
		},
	}
}

// GenerateExternalProvisionerClusterRoleBinding ...
func (c *IBMObjectCSI) GenerateExternalProvisionerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.GetResourceName(constants.ExternalProvisionerClusterRoleBinding),
			Labels: constants.CommonCSIResourceLabels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.GetResourceName(constants.CSIControllerServiceAccount),
				Namespace: c.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     constants.GetResourceName(constants.ExternalProvisionerClusterRole),
			APIGroup: constants.RbacAuthorizationAPIGroup,
		},
	}
}

// GenerateSCCForControllerClusterRole ...
func (c *IBMObjectCSI) GenerateSCCForControllerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.GetResourceName(constants.CSIControllerSCCClusterRole),
			Labels: constants.CommonCSIResourceLabels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{constants.SecurityOpenshiftAPIGroup},
				Resources:     []string{constants.SecurityContextConstraintsResource},
				ResourceNames: []string{"anyuid"},
				Verbs:         []string{"use"},
			},
		},
	}
}

// GenerateSCCForControllerClusterRoleBinding ...
func (c *IBMObjectCSI) GenerateSCCForControllerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.GetResourceName(constants.CSIControllerSCCClusterRoleBinding),
			Labels: constants.CommonCSIResourceLabels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.GetResourceName(constants.CSIControllerServiceAccount),
				Namespace: c.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     constants.GetResourceName(constants.CSIControllerSCCClusterRole),
			APIGroup: constants.RbacAuthorizationAPIGroup,
		},
	}
}

// GenerateSCCForNodeClusterRole ...
func (c *IBMObjectCSI) GenerateSCCForNodeClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.GetResourceName(constants.CSINodeSCCClusterRole),
			Labels: constants.CommonCSIResourceLabels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{constants.SecurityOpenshiftAPIGroup},
				Resources:     []string{constants.SecurityContextConstraintsResource},
				ResourceNames: []string{"privileged"},
				Verbs:         []string{"use"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.NodesResource},
				Verbs:     []string{constants.VerbGet},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.PersistentVolumesResource, constants.SecretsResource},
				Verbs:     []string{constants.VerbGet},
			},
			{
				APIGroups: []string{""},
				Resources: []string{constants.ConfigMapResource},
				Verbs:     []string{constants.VerbGet, constants.VerbList},
			},
		},
	}
}

// GenerateSCCForNodeClusterRoleBinding ...
func (c *IBMObjectCSI) GenerateSCCForNodeClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.GetResourceName(constants.CSINodeSCCClusterRoleBinding),
			Labels: constants.CommonCSIResourceLabels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.GetResourceName(constants.CSINodeServiceAccount),
				Namespace: c.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     constants.GetResourceName(constants.CSINodeSCCClusterRole),
			APIGroup: constants.RbacAuthorizationAPIGroup,
		},
	}
}

// Generates3fsSC ...
func (c *IBMObjectCSI) GenerateS3fsSC(reclaimPolicy corev1.PersistentVolumeReclaimPolicy, s3Provider string,
	region string, cosEndpoint string, cosStorageClass string) *storagev1.StorageClass {
	var storageClassName, locationConstraint string

	if reclaimPolicy == corev1.PersistentVolumeReclaimRetain {
		// "ibm-object-storage-standard-s3fs-retain"
		storageClassName = fmt.Sprintf("%s-%s-s3fs-%s", constants.StorageClassPrefix, cosStorageClass, constants.RetainPolicyTag)
	} else {
		// "ibm-object-storage-standard-s3fs"
		storageClassName = fmt.Sprintf("%s-%s-s3fs", constants.StorageClassPrefix, cosStorageClass)
	}

	if s3Provider == constants.S3ProviderIBM {
		locationConstraint = fmt.Sprintf("%s-%s", region, cosStorageClass)
	} else {
		locationConstraint = region
	}

	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   storageClassName,
			Labels: constants.CommonCSIResourceLabels,
		},
		Provisioner:   constants.DriverName,
		ReclaimPolicy: &reclaimPolicy,
		MountOptions: []string{
			"multipart_size=62",
			"max_dirty_data=51200",
			"parallel_count=8",
			"max_stat_cache_size=100000",
			"retries=5",
			"kernel_cache",
		},
		Parameters: map[string]string{
			"mounter":            "s3fs",
			"client":             "awss3",
			"cosEndpoint":        cosEndpoint,
			"locationConstraint": locationConstraint,
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.annotations['cos.csi.driver/secret']}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}
}

// GenerateRcloneSC ...
func (c *IBMObjectCSI) GenerateRcloneSC(reclaimPolicy corev1.PersistentVolumeReclaimPolicy, s3Provider string,
	region string, cosEndpoint string, cosStorageClass string) *storagev1.StorageClass {
	var storageClassName, locationConstraint string

	if reclaimPolicy == corev1.PersistentVolumeReclaimRetain {
		// "ibm-object-storage-standard-rclone-retain"
		storageClassName = fmt.Sprintf("%s-%s-rclone-%s", constants.StorageClassPrefix, cosStorageClass, constants.RetainPolicyTag)
	} else {
		// "ibm-object-storage-standard-rclone"
		storageClassName = fmt.Sprintf("%s-%s-rclone", constants.StorageClassPrefix, cosStorageClass)
	}

	if s3Provider == constants.S3ProviderIBM {
		locationConstraint = fmt.Sprintf("%s-%s", region, cosStorageClass)
	} else {
		locationConstraint = region
	}

	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   storageClassName,
			Labels: constants.CommonCSIResourceLabels,
		},
		Provisioner:   constants.DriverName,
		ReclaimPolicy: &reclaimPolicy,
		MountOptions: []string{
			"acl=private",
			"bucket_acl=private",
			"upload_cutoff=256Mi",
			"chunk_size=64Mi",
			"max_upload_parts=64",
			"upload_concurrency=20",
			"copy_cutoff=1Gi",
			"memory_pool_flush_time=30s",
			"disable_checksum=true",
		},
		Parameters: map[string]string{
			"mounter":            "rclone",
			"client":             "awss3",
			"cosEndpoint":        cosEndpoint,
			"locationConstraint": locationConstraint,
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.annotations['cos.csi.driver/secret']}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}
}
