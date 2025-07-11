package controllers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	fakecreate "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_create"
	fakecreatesc "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_create/storageclass"
	fakedelete "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_delete"
	fakeget "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_get"
	fakegetcsidriver "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_get/csidriver"
	fakegetsa "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_get/serviceaccount"
	fakelist "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_list"
	fakeupdate "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_update"
	fakeupdateibmobjcsi "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_update/ibmobjectcsi"
	crutils "github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util/common"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	defaultFSGroupPolicy = storagev1.FileFSGroupPolicy
	reclaimPolicyRetain  = corev1.PersistentVolumeReclaimRetain
	reclaimPolicyDelete  = corev1.PersistentVolumeReclaimDelete
	secrets              = crutils.GetImagePullSecrets(ibmObjectCSICR.Spec.ImagePullSecrets)

	operatorDeploymnet = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DeploymentName,
			Namespace: constants.CSIOperatorNamespace,
		},
	}

	ibmObjectCSIReconcileRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ibmObjectCSICRName,
			Namespace: TestNamespace,
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

	resources = v1alpha1.ResourcesSpec{
		Limits: v1alpha1.ReqLimits{
			CPU:    "40Mi",
			Memory: "200Mi",
		},
		Requests: v1alpha1.ReqLimits{
			CPU:    "40m",
			Memory: "40Mi",
		},
	}

	ibmObjectCSICR = &v1alpha1.IBMObjectCSI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ibmObjectCSICRName,
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.IBMObjectCSISpec{
			Controller: v1alpha1.IBMObjectCSIControllerSpec{
				Repository:      "icr.io/ibm/ibm-object-csi-driver",
				Tag:             "v1.0.2-alpha",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Affinity:        affinity,
				Resources:       resources,
			},
			Node: v1alpha1.IBMObjectCSINodeSpec{
				Repository:      "icr.io/ibm/ibm-object-csi-driver",
				Tag:             "v1.0.2-alpha",
				ImagePullPolicy: corev1.PullAlways,
				Affinity:        affinity,
				Tolerations: []corev1.Toleration{
					{
						Operator: corev1.TolerationOpExists,
					},
				},
				Resources:         resources,
				MaxVolumesPerNode: "0",
			},
			Sidecars: []v1alpha1.CSISidecar{
				{
					Name:            constants.CSINodeDriverRegistrar,
					Repository:      "k8s.gcr.io/sig-storage/csi-node-driver-registrar",
					Tag:             "v2.12.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Resources:       resources,
				},
				{
					Name:            constants.CSIProvisioner,
					Repository:      "k8s.gcr.io/sig-storage/csi-provisioner",
					Tag:             "v5.1.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Resources:       resources,
				},
				{
					Name:            constants.LivenessProbe,
					Repository:      "k8s.gcr.io/sig-storage/livenessprobe",
					Tag:             "v2.14.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Resources:       resources,
				},
			},
			ImagePullSecrets: []string{"secretName"},
			HealthPort:       9808,
		},
	}

	ibmObjectCSICRWithAWSProvider = &v1alpha1.IBMObjectCSI{
		ObjectMeta: ibmObjectCSICR.ObjectMeta,
		Spec: v1alpha1.IBMObjectCSISpec{
			Controller:       ibmObjectCSICR.Spec.Controller,
			Node:             ibmObjectCSICR.Spec.Node,
			Sidecars:         ibmObjectCSICR.Spec.Sidecars,
			ImagePullSecrets: ibmObjectCSICR.Spec.ImagePullSecrets,
			HealthPort:       ibmObjectCSICR.Spec.HealthPort,
			S3Provider:       constants.S3ProviderAWS,
			S3ProviderRegion: "us-east-2",
		},
	}

	ibmObjectCSICRWithDeletionTS = &v1alpha1.IBMObjectCSI{
		ObjectMeta: metav1.ObjectMeta{
			Name:              ibmObjectCSICRName,
			Namespace:         TestNamespace,
			Finalizers:        []string{ibmObjectCSIfinalizer},
			DeletionTimestamp: &currentTime,
		},
		Spec: v1alpha1.IBMObjectCSISpec{
			S3Provider:       constants.S3ProviderAWS,
			S3ProviderRegion: "us-east-2",
		},
	}

	ibmObjectCSICRWithFinaliser = &v1alpha1.IBMObjectCSI{
		ObjectMeta: metav1.ObjectMeta{
			Name:       ibmObjectCSICRName,
			Namespace:  TestNamespace,
			Finalizers: []string{ibmObjectCSIfinalizer},
		},
		Spec: ibmObjectCSICR.Spec,
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
			Name:        constants.GetResourceName(constants.CSINode),
			Namespace:   TestNamespace,
			Annotations: annotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Template: podTemplateSpec,
		},
	}

	controllerDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.GetResourceName(constants.CSIController),
			Namespace:   TestNamespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Template: podTemplateSpec,
		},
	}

	controllerPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerDeployment.Name + "-pod",
			Namespace: TestNamespace,
		},
	}

	csiDriver = &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:   constants.DriverName,
			Labels: map[string]string{"app.kubernetes.io/name": constants.ObjectCSIDriver},
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: util.False(),
			PodInfoOnMount: util.True(),
			FSGroupPolicy:  &defaultFSGroupPolicy,
		},
	}

	controllerSA = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GetResourceName(constants.CSIControllerServiceAccount),
			Namespace: TestNamespace,
		},
		ImagePullSecrets: secrets,
	}

	nodeSA = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GetResourceName(constants.CSINodeServiceAccount),
			Namespace: TestNamespace,
		},
		ImagePullSecrets: secrets,
	}

	externalProvisionerCRB = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.GetResourceName(constants.ExternalProvisionerClusterRoleBinding),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.GetResourceName(constants.CSIControllerServiceAccount),
				Namespace: TestNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     constants.GetResourceName(constants.ExternalProvisionerClusterRole),
			APIGroup: constants.RbacAuthorizationAPIGroup,
		},
	}

	controllerSCCCRB = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.GetResourceName(constants.CSIControllerSCCClusterRoleBinding),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.GetResourceName(constants.CSIControllerServiceAccount),
				Namespace: TestNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     constants.GetResourceName(constants.CSIControllerSCCClusterRole),
			APIGroup: constants.RbacAuthorizationAPIGroup,
		},
	}

	nodeSCCCRB = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.GetResourceName(constants.CSINodeSCCClusterRoleBinding),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.GetResourceName(constants.CSINodeServiceAccount),
				Namespace: TestNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     constants.GetResourceName(constants.CSINodeSCCClusterRole),
			APIGroup: constants.RbacAuthorizationAPIGroup,
		},
	}

	externalProvisionerCR = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.GetResourceName(constants.ExternalProvisionerClusterRole),
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
				Verbs:     []string{constants.VerbGet, constants.VerbList, constants.VerbWatch, constants.VerbCreate, constants.VerbDelete},
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
		},
	}

	controllerSCCCR = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.GetResourceName(constants.CSIControllerSCCClusterRole),
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

	nodeSCCCR = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.GetResourceName(constants.CSINodeSCCClusterRole),
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
		},
	}

	addonConfigMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ParamsConfigMap,
			Namespace: constants.ParamsConfigMapNamespace,
		},
		Data: map[string]string{
			"maxVolumesPerNode": "0",
		},
	}

	addonConfigMapWithUpdatedData = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ParamsConfigMap,
			Namespace: constants.ParamsConfigMapNamespace,
		},
		Data: map[string]string{
			"maxVolumesPerNode":    "10",
			"CSINodeCPURequest":    "5m",
			"CSINodeMemoryRequest": "5Mi",
			"CSINodeCPULimit":      "50m",
			"CSINodeMemoryLimit":   "50Mi",
		},
	}

	rCloneSC = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-standard-rclone", constants.StorageClassPrefix),
			Labels: constants.CommonCSIResourceLabels,
		},
		Provisioner:   constants.DriverName,
		ReclaimPolicy: &reclaimPolicyDelete,
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
			"cosEndpoint":        "https://s3.us-east-2.amazonaws.com",
			"locationConstraint": "us-east-2",
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.annotations['cos.csi.driver/secret']}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}

	rCloneRetainSC = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-standard-rclone-retain", constants.StorageClassPrefix),
			Labels: constants.CommonCSIResourceLabels,
		},
		Provisioner:   constants.DriverName,
		ReclaimPolicy: &reclaimPolicyRetain,
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
			"cosEndpoint":        "https://s3.us-east-2.amazonaws.com",
			"locationConstraint": "us-east-2",
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.annotations['cos.csi.driver/secret']}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}

	s3fsSC = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-standard-s3fs", constants.StorageClassPrefix),
			Labels: constants.CommonCSIResourceLabels,
		},
		Provisioner:   constants.DriverName,
		ReclaimPolicy: &reclaimPolicyDelete,
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
			"cosEndpoint":        "https://s3.us-east-2.amazonaws.com",
			"locationConstraint": "us-east-2",
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.annotations['cos.csi.driver/secret']}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}

	s3fsRetainSC = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-standard-s3fs-retain", constants.StorageClassPrefix),
			Labels: constants.CommonCSIResourceLabels,
		},
		Provisioner:   constants.DriverName,
		ReclaimPolicy: &reclaimPolicyRetain,
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
			"cosEndpoint":        "https://s3.us-east-2.amazonaws.com",
			"locationConstraint": "us-east-2",
			"csi.storage.k8s.io/node-publish-secret-name":      "${pvc.annotations['cos.csi.driver/secret']}",
			"csi.storage.k8s.io/node-publish-secret-namespace": "${pvc.namespace}",
		},
	}
)

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
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMap,
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
				operatorDeploymnet,
				ibmObjectCSICRWithAWSProvider,
				addonConfigMap,
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
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerDeployment.Name + "-pod2",
						Namespace: TestNamespace,
					},
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				statusSubRes := ibmObjectCSICR
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).WithStatusSubresource(statusSubRes).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: Successfully updated status in IBMObjectCSI CR after validating if pod images are in sync",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMap,
				controllerSA,
				nodeSA,
				&appsv1.Deployment{
					ObjectMeta: controllerDeployment.ObjectMeta,
					Spec:       controllerDeployment.Spec,
					Status: appsv1.DeploymentStatus{
						Replicas:      1,
						ReadyReplicas: 0,
					},
				},
				&corev1.Pod{
					ObjectMeta: controllerPod.ObjectMeta,
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.ControllerContainerName,
								Image: ibmObjectCSICR.Spec.Controller.Repository + ":" + ibmObjectCSICR.Spec.Controller.Tag,
							},
							{
								Name:  constants.CSIProvisioner,
								Image: ibmObjectCSICR.Spec.Sidecars[1].Repository + ":" + ibmObjectCSICR.Spec.Sidecars[1].Tag,
							},
							{
								Name:  constants.LivenessProbe,
								Image: ibmObjectCSICR.Spec.Sidecars[2].Repository + ":" + ibmObjectCSICR.Spec.Sidecars[2].Tag,
							},
						},
					},
				},
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
				operatorDeploymnet,
				ibmObjectCSICRWithDeletionTS,
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
			testCaseName: "Negative: Failed to Add Finalizer in IBMObjectCSI CR",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Negative: Failed to get configmap",
			objects: []runtime.Object{
				ibmObjectCSICR,
				addonConfigMap,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeget.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: configmap not found",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{RequeueAfter: 5 * time.Second},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to update IBMObjectCSI CR as per updated configmap data",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMapWithUpdatedData,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Positive: IBMObjectCSI CR updated as per updated configmap data",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMapWithUpdatedData,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to create CSI driver while reconciling",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMap,
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
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMap,
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
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMap,
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
				addonConfigMap,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(NotFoundError),
		},
		{
			testCaseName: "Failed to get controller deployment while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICR,
				addonConfigMap,
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
				operatorDeploymnet,
				ibmObjectCSICRWithFinaliser,
				addonConfigMap,
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
				operatorDeploymnet,
				ibmObjectCSICR,
				addonConfigMap,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetsa.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to get controller pod while reconciling",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICRWithFinaliser,
				addonConfigMap,
				csiNode,
				controllerDeployment,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakelist.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
		{
			testCaseName: "Negative: Controller pod not found while reconciling",
			objects: []runtime.Object{
				ibmObjectCSICRWithFinaliser,
				addonConfigMap,
				csiNode,
				controllerDeployment,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(NotFoundError),
		},
		{
			testCaseName: "Negative: Failed to sync CSI Controller",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICRWithFinaliser,
				addonConfigMap,
				controllerSA,
				nodeSA,
				&appsv1.Deployment{
					ObjectMeta: controllerDeployment.ObjectMeta,
					Spec:       controllerDeployment.Spec,
					Status: appsv1.DeploymentStatus{
						Replicas:      1,
						ReadyReplicas: 0,
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
			testCaseName: "Negative: Failed to sync CSI Node",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICRWithFinaliser,
				addonConfigMap,
				controllerSA,
				nodeSA,
				csiNode,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Negative: Failed to create storage class while reconciling",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICRWithAWSProvider,
				addonConfigMap,
				csiNode,
				controllerDeployment,
				controllerPod,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakecreatesc.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(CreateError),
		},
		{
			testCaseName: "Negative: Failed to update status in IBMObjectCSI CR",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICRWithFinaliser,
				addonConfigMap,
				controllerSA,
				nodeSA,
				&appsv1.Deployment{
					ObjectMeta: controllerDeployment.ObjectMeta,
					Spec:       controllerDeployment.Spec,
					Status: appsv1.DeploymentStatus{
						Replicas:      1,
						ReadyReplicas: 0,
					},
				},
				&corev1.Pod{
					ObjectMeta: controllerPod.ObjectMeta,
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: constants.ControllerContainerName,
							},
							{
								Name: constants.CSIProvisioner,
							},
							{
								Name: constants.LivenessProbe,
							},
						},
					},
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				statusSubRes := ibmObjectCSICRWithFinaliser
				return fakeupdateibmobjcsi.NewClientBuilder().WithRuntimeObjects(objs...).WithStatusSubresource(statusSubRes).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
		{
			testCaseName: "Negative: IBMObjectCSI CR is deleted and failed to delete cluster role binding",
			objects: []runtime.Object{
				operatorDeploymnet,
				ibmObjectCSICRWithDeletionTS,
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
				operatorDeploymnet,
				ibmObjectCSICRWithDeletionTS,
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
				operatorDeploymnet,
				ibmObjectCSICRWithDeletionTS,
				rCloneSC,
				rCloneRetainSC,
				s3fsSC,
				s3fsRetainSC,
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
				operatorDeploymnet,
				ibmObjectCSICRWithDeletionTS,
				csiDriver,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakedelete.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(DeleteError),
		},
		{
			testCaseName: "Negative: Failed to get CSI driver while deleting",
			objects: []runtime.Object{
				operatorDeploymnet,
				&v1alpha1.IBMObjectCSI{
					ObjectMeta: ibmObjectCSICRWithDeletionTS.ObjectMeta,
					Spec:       ibmObjectCSICR.Spec,
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetcsidriver.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to remove finaliser from IBMObjectCSI CR",
			objects: []runtime.Object{
				operatorDeploymnet,
				&v1alpha1.IBMObjectCSI{
					ObjectMeta: ibmObjectCSICRWithDeletionTS.ObjectMeta,
					Spec:       ibmObjectCSICR.Spec,
				},
				csiDriver,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeupdate.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(UpdateError),
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			TestLog.Info("Testcase being executed", "testcase", testcase.testCaseName)

			scheme := setupScheme()
			client := testcase.clientFunc(testcase.objects)

			ibmObjectCSIReconciler := &IBMObjectCSIReconciler{
				Client: client,
				Scheme: scheme,
				// Recorder:         record.NewFakeRecorder(0),
				ControllerHelper: common.NewControllerHelper(client, TestLog),
			}

			res, err := ibmObjectCSIReconciler.Reconcile(TestCtx, ibmObjectCSIReconcileRequest)
			TestLog.Info("Testcase return values", "result", res, "error", err)

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

func TestIBMObjectCSISetupWithManager(t *testing.T) {
	t.Run("Positive: Successful", func(t *testing.T) {
		ibmObjectCSIReconciler := &IBMObjectCSIReconciler{}
		ibmObjectCSIReconciler.SetupWithManager(nil) // #nosec G104 Skip error
	})
}
