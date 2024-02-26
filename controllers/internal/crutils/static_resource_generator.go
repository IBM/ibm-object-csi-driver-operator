package crutils

import (
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/util/boolptr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	securityOpenshiftApiGroup                string = "security.openshift.io"
	storageApiGroup                          string = "storage.k8s.io"
	rbacAuthorizationApiGroup                string = "rbac.authorization.k8s.io"
	storageClassesResource                   string = "storageclasses"
	persistentVolumesResource                string = "persistentvolumes"
	persistentVolumeClaimsResource           string = "persistentvolumeclaims"
	persistentVolumeClaimsStatusResource     string = "persistentvolumeclaims/status"
	persistentVolumeClaimsFinalizersResource string = "persistentvolumeclaims/finalizers"
	podsResource                             string = "pods"
	eventsResource                           string = "events"
	nodesResource                            string = "nodes"
	csiNodesResource                         string = "csinodes"
	secretsResource                          string = "secrets"
	securityContextConstraintsResource       string = "securitycontextconstraints"
	verbGet                                  string = "get"
	verbList                                 string = "list"
	verbWatch                                string = "watch"
	verbCreate                               string = "create"
	verbUpdate                               string = "update"
	verbPatch                                string = "patch"
	verbDelete                               string = "delete"
)

func (c *IBMObjectCSI) GenerateCSIDriver() *storagev1.CSIDriver {
	defaultFSGroupPolicy := storagev1.FileFSGroupPolicy
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.DriverName,
			Labels: map[string]string{"app.kubernetes.io/name": "ibm-object-csi"},
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: boolptr.False(),
			PodInfoOnMount: boolptr.True(),
			FSGroupPolicy:  &defaultFSGroupPolicy,
		},
	}
}

func (c *IBMObjectCSI) GenerateControllerServiceAccount() *corev1.ServiceAccount {
	return getServiceAccount(c, config.CSIControllerServiceAccount)
}

func (c *IBMObjectCSI) GenerateNodeServiceAccount() *corev1.ServiceAccount {
	return getServiceAccount(c, config.CSINodeServiceAccount)
}

func getServiceAccount(c *IBMObjectCSI, serviceAccountResourceName config.ResourceName) *corev1.ServiceAccount {
	secrets := getImagePullSecrets(c.Spec.ImagePullSecrets)
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GetNameForResource(serviceAccountResourceName, c.Name),
			Namespace: c.Namespace,
			Labels:    c.GetLabels(),
		},
		ImagePullSecrets: secrets,
	}
}

func (c *IBMObjectCSI) GenerateExternalProvisionerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.ExternalProvisionerClusterRole, c.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{secretsResource},
				Verbs:     []string{verbGet, verbList},
			},
			{
				APIGroups: []string{""},
				Resources: []string{persistentVolumesResource},
				Verbs:     []string{verbGet, verbList, verbWatch, verbCreate, verbDelete},
			},
			{
				APIGroups: []string{""},
				Resources: []string{persistentVolumeClaimsResource},
				Verbs:     []string{verbGet, verbList, verbWatch, verbUpdate},
			},
			{
				APIGroups: []string{storageApiGroup},
				Resources: []string{storageClassesResource},
				Verbs:     []string{verbGet, verbList, verbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{eventsResource},
				Verbs:     []string{verbList, verbWatch, verbCreate, verbUpdate, verbPatch},
			},
			{
				APIGroups: []string{storageApiGroup},
				Resources: []string{csiNodesResource},
				Verbs:     []string{verbGet, verbList, verbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{nodesResource},
				Verbs:     []string{verbGet, verbList, verbWatch},
			},
		},
	}
}

func (c *IBMObjectCSI) GenerateExternalProvisionerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.ExternalProvisionerClusterRoleBinding, c.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.GetNameForResource(config.CSIControllerServiceAccount, c.Name),
				Namespace: c.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     config.GetNameForResource(config.ExternalProvisionerClusterRole, c.Name),
			APIGroup: rbacAuthorizationApiGroup,
		},
	}
}

func (c *IBMObjectCSI) GenerateSCCForControllerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSIControllerSCCClusterRole, c.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{securityOpenshiftApiGroup},
				Resources:     []string{securityContextConstraintsResource},
				ResourceNames: []string{"anyuid"},
				Verbs:         []string{"use"},
			},
		},
	}
}

func (c *IBMObjectCSI) GenerateSCCForControllerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSIControllerSCCClusterRoleBinding, c.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.GetNameForResource(config.CSIControllerServiceAccount, c.Name),
				Namespace: c.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     config.GetNameForResource(config.CSIControllerSCCClusterRole, c.Name),
			APIGroup: rbacAuthorizationApiGroup,
		},
	}
}

func (c *IBMObjectCSI) GenerateSCCForNodeClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSINodeSCCClusterRole, c.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{securityOpenshiftApiGroup},
				Resources:     []string{securityContextConstraintsResource},
				ResourceNames: []string{"privileged"},
				Verbs:         []string{"use"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{nodesResource},
				Verbs:     []string{verbGet},
			},
		},
	}
}

func (c *IBMObjectCSI) GenerateSCCForNodeClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSINodeSCCClusterRoleBinding, c.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.GetNameForResource(config.CSINodeServiceAccount, c.Name),
				Namespace: c.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     config.GetNameForResource(config.CSINodeSCCClusterRole, c.Name),
			APIGroup: rbacAuthorizationApiGroup,
		},
	}
}
func (c *IBMObjectCSI) Generates3fsSC() *storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.S3fsStorageClass, c.Name),
		},
		Provisioner:   config.DriverName,
		ReclaimPolicy: &reclaimPolicy,
		MountOptions: []string{
			"multipart_size=62",
			"max_dirty_data=51200",
			"parallel_count=8",
			"max_stat_cache_size=100000",
			"retries=5",
			"cache=kernel_cache",
		},
		Parameters: map[string]string{
			"mounter": "s3fs",
			"client":  "awss3",
			"csi.storage.k8s.io/provisioner-secret-name":       "${pvc.name}",
			"csi.storage.k8s.io/provisioner-secret-namespace":  "${pvc.namespace}",
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.name}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}
}

func (c *IBMObjectCSI) GenerateRcloneSC() *storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.RcloneStorageClass, c.Name),
		},
		Provisioner:   config.DriverName,
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
			"mounter": "rclone",
			"client":  "awss3",
			"csi.storage.k8s.io/provisioner-secret-name":       "${pvc.name}",
			"csi.storage.k8s.io/provisioner-secret-namespace":  "${pvc.namespace}",
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.name}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}
}
