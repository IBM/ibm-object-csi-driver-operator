package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	csiv1alpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	fakecreate "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_create"
	fakedelete "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_delete"
	fakeget "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get"
	fakegetcsidriver "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get/csidriver"
	fakegetsa "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get/serviceaccount"
	fakeupdate "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_update"
	crutils "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/util/common"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/util/boolptr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CreateError = "failed to create"
	DeleteError = "failed to delete"
	GetError    = "failed to get"
	UpdateError = "failed to update"

	NotFoundError = "not found"
)

var (
	testLog = log.Log.WithName("ibmobjectcsi_controller_test")
	testCtx = context.TODO()

	currentTime = metav1.Now()

	ibmObjectCSICRName      = "test-csi-cr"
	ibmObjectCSICRNamespace = "test-namespace"
	ibmObjectCSIfinalizer   = "ibmobjectcsi.csi.ibm.com"

	defaultFSGroupPolicy = storagev1.FileFSGroupPolicy
	reclaimPolicy        = corev1.PersistentVolumeReclaimRetain
	secrets              = crutils.GetImagePullSecrets(ibmObjectCSICR.Spec.ImagePullSecrets)

	ibmObjectCSIReconcileRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ibmObjectCSICRName,
			Namespace: ibmObjectCSICRNamespace,
		},
	}

	affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"amd64"},
							},
						},
					},
				},
			},
		},
	}

	ibmObjectCSICR = &csiv1alpha1.IBMObjectCSI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ibmObjectCSICRName,
			Namespace: ibmObjectCSICRNamespace,
		},
		Spec: csiv1alpha1.IBMObjectCSISpec{
			Controller: csiv1alpha1.IBMObjectCSIControllerSpec{
				Repository:      "icr.io/ibm/ibm-object-csi-driver",
				Tag:             "v1.0.1-alpha",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Affinity:        affinity,
			},
			Node: csiv1alpha1.IBMObjectCSINodeSpec{
				Repository:      "icr.io/ibm/ibm-object-csi-driver",
				Tag:             "v1.0.1-alpha",
				ImagePullPolicy: corev1.PullAlways,
				Affinity:        affinity,
				Tolerations: []corev1.Toleration{
					{
						Operator: corev1.TolerationOpExists,
					},
				},
			},
			Sidecars: []csiv1alpha1.CSISidecar{
				{
					Name:            "csi-node-driver-registrar",
					Repository:      "k8s.gcr.io/sig-storage/csi-node-driver-registrar",
					Tag:             "v2.6.3",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:            "csi-provisioner",
					Repository:      "k8s.gcr.io/sig-storage/csi-provisioner",
					Tag:             "v3.4.1",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:            "livenessprobe",
					Repository:      "k8s.gcr.io/sig-storage/livenessprobe",
					Tag:             "v2.9.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
			ImagePullSecrets: []string{"secretName"},
			HealthPort:       9808,
		},
	}

	ibmObjectCSICR_WithDeletionTS = &csiv1alpha1.IBMObjectCSI{
		ObjectMeta: metav1.ObjectMeta{
			Name:              ibmObjectCSICRName,
			Namespace:         ibmObjectCSICRNamespace,
			Finalizers:        []string{ibmObjectCSIfinalizer},
			DeletionTimestamp: &currentTime,
		},
	}

	annotations = map[string]string{
		"app": "cos-s3-csi-driver",
	}

	podTemplateSpec = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
		},
	}

	csiNode = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.GetNameForResource(config.CSINode, ibmObjectCSICRName),
			Namespace:   ibmObjectCSICRNamespace,
			Annotations: annotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Template: podTemplateSpec,
		},
	}

	controllerDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.GetNameForResource(config.CSIController, ibmObjectCSICRName),
			Namespace:   ibmObjectCSICRNamespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Template: podTemplateSpec,
		},
	}

	controllerPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-0", controllerDeployment.Name),
			Namespace: ibmObjectCSICRNamespace,
		},
	}

	csiDriver = &storagev1.CSIDriver{
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

	controllerSA = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GetNameForResource(config.CSIControllerServiceAccount, ibmObjectCSICRName),
			Namespace: ibmObjectCSICRNamespace,
		},
		ImagePullSecrets: secrets,
	}

	nodeSA = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GetNameForResource(config.CSINodeServiceAccount, ibmObjectCSICRName),
			Namespace: ibmObjectCSICRNamespace,
		},
		ImagePullSecrets: secrets,
	}

	externalProvisionerCRB = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.ExternalProvisionerClusterRoleBinding, ibmObjectCSICRName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.GetNameForResource(config.CSIControllerServiceAccount, ibmObjectCSICRName),
				Namespace: ibmObjectCSICRNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     config.GetNameForResource(config.ExternalProvisionerClusterRole, ibmObjectCSICRName),
			APIGroup: config.RbacAuthorizationApiGroup,
		},
	}

	controllerSCCCRB = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSIControllerSCCClusterRoleBinding, ibmObjectCSICRName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.GetNameForResource(config.CSIControllerServiceAccount, ibmObjectCSICRName),
				Namespace: ibmObjectCSICRNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     config.GetNameForResource(config.CSIControllerSCCClusterRole, ibmObjectCSICRName),
			APIGroup: config.RbacAuthorizationApiGroup,
		},
	}

	nodeSCCCRB = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSINodeSCCClusterRoleBinding, ibmObjectCSICRName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.GetNameForResource(config.CSINodeServiceAccount, ibmObjectCSICRName),
				Namespace: ibmObjectCSICRNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     config.GetNameForResource(config.CSINodeSCCClusterRole, ibmObjectCSICRName),
			APIGroup: config.RbacAuthorizationApiGroup,
		},
	}

	externalProvisionerCR = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.ExternalProvisionerClusterRole, ibmObjectCSICRName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{config.SecretsResource},
				Verbs:     []string{config.VerbGet, config.VerbList},
			},
			{
				APIGroups: []string{""},
				Resources: []string{config.PersistentVolumesResource},
				Verbs:     []string{config.VerbGet, config.VerbList, config.VerbWatch, config.VerbCreate, config.VerbDelete},
			},
			{
				APIGroups: []string{""},
				Resources: []string{config.PersistentVolumeClaimsResource},
				Verbs:     []string{config.VerbGet, config.VerbList, config.VerbWatch, config.VerbUpdate},
			},
			{
				APIGroups: []string{config.StorageApiGroup},
				Resources: []string{config.StorageClassesResource},
				Verbs:     []string{config.VerbGet, config.VerbList, config.VerbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{config.EventsResource},
				Verbs:     []string{config.VerbList, config.VerbWatch, config.VerbCreate, config.VerbUpdate, config.VerbPatch},
			},
			{
				APIGroups: []string{config.StorageApiGroup},
				Resources: []string{config.CsiNodesResource},
				Verbs:     []string{config.VerbGet, config.VerbList, config.VerbWatch},
			},
			{
				APIGroups: []string{""},
				Resources: []string{config.NodesResource},
				Verbs:     []string{config.VerbGet, config.VerbList, config.VerbWatch},
			},
		},
	}

	controllerSCCCR = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSIControllerSCCClusterRole, ibmObjectCSICRName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{config.SecurityOpenshiftApiGroup},
				Resources:     []string{config.SecurityContextConstraintsResource},
				ResourceNames: []string{"anyuid"},
				Verbs:         []string{"use"},
			},
		},
	}

	nodeSCCCR = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.CSINodeSCCClusterRole, ibmObjectCSICRName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{config.SecurityOpenshiftApiGroup},
				Resources:     []string{config.SecurityContextConstraintsResource},
				ResourceNames: []string{"privileged"},
				Verbs:         []string{"use"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{config.NodesResource},
				Verbs:     []string{config.VerbGet},
			},
		},
	}

	rCloneSC = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.RcloneStorageClass, ibmObjectCSICRName),
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

	s3fsSC = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetNameForResource(config.S3fsStorageClass, ibmObjectCSICRName),
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
)

func TestMain(m *testing.M) {
	os.Setenv(config.EnvNameIBMObjectCSICrYaml, "../config/samples/csi_v1alpha1_ibmobjectcsi.yaml")
	err := config.LoadDefaultsOfIBMObjectCSI()
	if err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func setupScheme() *runtime.Scheme {
	s := scheme.Scheme
	_ = csiv1alpha1.AddToScheme(s)
	return s
}

func TestIBMObjectCSIReconcile(t *testing.T) {
	testCases := []struct {
		testCaseName string
		objects      []runtime.Object
		clientFunc   func(objs []runtime.Object) client.WithWatch
		expectedResp reconcile.Result
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			objects: []runtime.Object{
				ibmObjectCSICR,
				csiNode,
				controllerDeployment,
				controllerPod,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				statusSubRes := ibmObjectCSICR
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).WithStatusSubresource(statusSubRes).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: IBMObjectCSI CR not found",
			objects:      []runtime.Object{},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: Sync controller deployment & pod containers and update status in IBMObjectCSI CR",
			objects: []runtime.Object{
				ibmObjectCSICR,
				csiNode,
				&appsv1.Deployment{
					ObjectMeta: controllerDeployment.ObjectMeta,
					Spec:       controllerDeployment.Spec,
					Status: appsv1.DeploymentStatus{
						Replicas:      3,
						ReadyReplicas: 0,
					},
				},
				controllerPod,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				statusSubRes := ibmObjectCSICR
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).WithStatusSubresource(statusSubRes).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: Successfully removed finaliser from IBMObjectCSI CR",
			objects: []runtime.Object{
				ibmObjectCSICR_WithDeletionTS,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to get IBMObjectCSI CR",
			objects:      []runtime.Object{},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeget.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to update IBMObjectCSI CR after defaulting",
			objects: []runtime.Object{
				&csiv1alpha1.IBMObjectCSI{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ibmObjectCSICRName,
						Namespace: ibmObjectCSICRNamespace,
					},
					Spec: csiv1alpha1.IBMObjectCSISpec{
						Controller: csiv1alpha1.IBMObjectCSIControllerSpec{
							Repository:      "icr.io/ibm/ibm-object-csi-driver",
							Tag:             "v1.0.1",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						Node: csiv1alpha1.IBMObjectCSINodeSpec{
							Repository:      "icr.io/ibm/ibm-object-csi-driver",
							Tag:             "v1.0.1",
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Negative: Failed to Add Finalizer in IBMObjectCSI CR",
			objects: []runtime.Object{
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Negative: Failed to create CSI driver while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakecreate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(CreateError),
		},
		{
			testCaseName: "Negative: Failed to get CSI driver while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetcsidriver.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to create service account while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
				csiDriver,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakecreate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(CreateError),
		},
		{
			testCaseName: "Failed to get node daemon set while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(NotFoundError),
		},
		{
			testCaseName: "Failed to get controller deployment set while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
				csiNode,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(NotFoundError),
		},
		{
			testCaseName: "Negative: Failed to restart node while reconciling",
			objects: []runtime.Object{
				&csiv1alpha1.IBMObjectCSI{
					ObjectMeta: metav1.ObjectMeta{
						Name:       ibmObjectCSICRName,
						Namespace:  ibmObjectCSICRNamespace,
						Finalizers: []string{ibmObjectCSIfinalizer},
					},
					Spec: ibmObjectCSICR.Spec,
				},
				csiNode,
				controllerDeployment,
				controllerPod,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Failed to get service account while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetsa.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		// {
		// 	testCaseName: "Negative: Failed to update status in IBMObjectCSI CR",
		// 	objects: []runtime.Object{
		// 		ibmObjectCSICR,
		// 		controllerSA,
		// 		nodeSA,
		// &appsv1.Deployment{
		// 	ObjectMeta: controllerDeployment.ObjectMeta,
		// 	Spec:       controllerDeployment.Spec,
		// 	Status: appsv1.DeploymentStatus{
		// 		Replicas:      3,
		// 		ReadyReplicas: 0,
		// 	},
		// },
		// &corev1.Pod{
		// 	ObjectMeta: controllerPod.ObjectMeta,
		// 	Spec: corev1.PodSpec{
		// 		Containers: []corev1.Container{},
		// 	},
		// },
		// 	},
		// 	clientFunc: func(objs []runtime.Object) client.WithWatch {
		// 		fmt.Println("------", "IN this test case")
		// 	statusSubRes := ibmObjectCSICR
		// 	return fake.NewClientBuilder().WithRuntimeObjects(objs...).WithStatusSubresource(statusSubRes).Build()
		// 		return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
		// 	},
		// 	expectedResp: reconcile.Result{},
		// 	expectedErr:  nil,
		// },
		{
			testCaseName: "Negative: IBMObjectCSI CR is deleted and failed to delete cluster role binding",
			objects: []runtime.Object{
				ibmObjectCSICR_WithDeletionTS,
				externalProvisionerCRB,
				controllerSCCCRB,
				nodeSCCCRB,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakedelete.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(DeleteError),
		},
		{
			testCaseName: "Negative: IBMObjectCSI CR is deleted and failed to delete cluster role",
			objects: []runtime.Object{
				ibmObjectCSICR_WithDeletionTS,
				externalProvisionerCR,
				controllerSCCCR,
				nodeSCCCR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakedelete.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(DeleteError),
		},
		{
			testCaseName: "Negative: IBMObjectCSI CR is deleted and failed to delete storage class",
			objects: []runtime.Object{
				ibmObjectCSICR_WithDeletionTS,
				rCloneSC,
				s3fsSC,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakedelete.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(DeleteError),
		},
		{
			testCaseName: "Negative: IBMObjectCSI CR is deleted and failed to delete CSI driver",
			objects: []runtime.Object{
				ibmObjectCSICR_WithDeletionTS,
				csiDriver,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakedelete.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(DeleteError),
		},
		{
			testCaseName: "Negative: Failed to remove finaliser from IBMObjectCSI CR",
			objects: []runtime.Object{
				&csiv1alpha1.IBMObjectCSI{
					ObjectMeta: ibmObjectCSICR_WithDeletionTS.ObjectMeta,
					Spec:       ibmObjectCSICR.Spec,
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		// {
		// 	testCaseName: "Negative: Failed to get CSI driver while deleting",
		// 	objects:      []runtime.Object{},
		// 	clientFunc: func(objs []runtime.Object) client.WithWatch {
		// 		return fakegetcsidriver.NewClientBuilder().WithRuntimeObjects(objs...).Build()
		// 	},
		// 	expectedResp: reconcile.Result{},
		// 	expectedErr:  nil,
		// },
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			testLog.Info("Testcase being executed", "testcase", testcase.testCaseName)

			scheme := setupScheme()
			client := testcase.clientFunc(testcase.objects)

			ibmObjectCSIReconciler := &IBMObjectCSIReconciler{
				Client: client,
				Scheme: scheme,
				// Recorder: record.NewFakeRecorder(0),
				ControllerHelper: common.NewControllerHelper(client),
			}

			res, err := ibmObjectCSIReconciler.Reconcile(testCtx, ibmObjectCSIReconcileRequest)
			testLog.Info("Testcase return values", "result", res, "error", err)

			assert.Equal(t, testcase.expectedResp, res)

			if testcase.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testcase.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
