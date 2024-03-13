package controllers

import (
	"errors"
	"os"
	"testing"

	"github.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	fakedelete "github.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_delete"
	fakeget "github.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get"
	fakegetdeploy "github.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get/deployment"
	fakegetpvc "github.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get/pvc"
	fakelist "github.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_list"
	fakelistnodeserverpod "github.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_list/nodeserverpod"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeK8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	recoverStaleVolumeReconcileRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      recoverStaleVolCRName,
			Namespace: recoverStaleVolCRNamespace,
		},
	}

	recoverStaleVolumeCR = &v1alpha1.RecoverStaleVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoverStaleVolCRName,
			Namespace: recoverStaleVolCRNamespace,
		},
		Spec: v1alpha1.RecoverStaleVolumeSpec{
			NoOfLogLines: int64(100),
			Deployment: []v1alpha1.DeploymentData{
				{
					DeploymentName: testDeploymentName,
				},
				{
					DeploymentName:      "deployment-not-existing",
					DeploymentNamespace: testDeploymentNamespace,
				},
			},
		},
	}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName,
			Namespace: testDeploymentNamespace,
		},
		Spec: appsv1.DeploymentSpec{},
	}

	deploymentPod1 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName + "-pod1",
			Namespace: testDeploymentNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode1,
			Volumes: []corev1.Volume{
				{
					Name: testPVName1,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: testPVCName1,
						},
					},
				},
				{
					Name: "pvc-not-existing",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pv-not-existing",
						},
					},
				},
			},
		},
	}

	deploymentPod2 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName + "-pod2",
			Namespace: testDeploymentNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode1,
			Volumes: []corev1.Volume{
				{
					Name: testPVName2,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: testPVCName2,
						},
					},
				},
			},
		},
	}

	pvc1 = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPVCName1,
			Namespace: testDeploymentNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &testStorageClassName,
			VolumeName:       testPVName1,
		},
	}

	pvc2 = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPVCName2,
			Namespace: testDeploymentNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &testStorageClassName,
			VolumeName:       testPVName2,
		},
	}

	nodeServerPod1 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiNodePodPrefix + "-pod1",
			Namespace: csiOperatorNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode1,
		},
	}

	nodeServerPod2 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiNodePodPrefix + "-pod2",
			Namespace: csiOperatorNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode2,
		},
	}

	nodeServerPod3 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiNodePodPrefix + "-pod3",
			Namespace: csiOperatorNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode3,
		},
	}
)

func TestRecoverStaleVolumeReconcile(t *testing.T) {
	testCases := []struct {
		testCaseName   string
		objects        []runtime.Object
		clientFunc     func(objs []runtime.Object) client.WithWatch
		kubeClientFunc func() (*KubernetesClient, error)
		expectedResp   reconcile.Result
		expectedErr    error
	}{
		{
			testCaseName: "Positive: Successful",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
				deploymentPod2,
				pvc1,
				pvc2,
				nodeServerPod1,
				nodeServerPod2,
				nodeServerPod3,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return &KubernetesClient{
					Clientset: fakeK8s.NewSimpleClientset(),
				}, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: reconcileTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: RecoverStaleVolume CR not found",
			objects:      []runtime.Object{},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: No workload is passed to watch",
			objects: []runtime.Object{
				&v1alpha1.RecoverStaleVolume{
					ObjectMeta: recoverStaleVolumeCR.ObjectMeta,
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: reconcileTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: Node Server Pods not found",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
				pvc1,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: reconcileTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to get RecoverStaleVolume CR",
			objects:      []runtime.Object{},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeget.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Deployment Name is missing",
			objects: []runtime.Object{
				&v1alpha1.RecoverStaleVolume{
					ObjectMeta: recoverStaleVolumeCR.ObjectMeta,
					Spec: v1alpha1.RecoverStaleVolumeSpec{
						Deployment: []v1alpha1.DeploymentData{
							{
								DeploymentName: "",
							},
						},
					},
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: reconcileTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to get deployment",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetdeploy.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to get deployment pods list",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakelist.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
		{
			testCaseName: "Megative: Failed to get pvc",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetpvc.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to get node-server-pods list",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
				pvc1,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakelistnodeserverpod.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
		{
			testCaseName: "Negative: Failed to get In Cluster Config",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
				pvc1,
				nodeServerPod1,
				nodeServerPod2,
				nodeServerPod3,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, rest.ErrNotInCluster
			},
			expectedResp: reconcile.Result{},
			expectedErr:  rest.ErrNotInCluster,
		},
		{
			testCaseName: "Negative: Failed to delete pod",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
				pvc1,
				nodeServerPod1,
				nodeServerPod2,
				nodeServerPod3,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakedelete.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return &KubernetesClient{
					Clientset: fakeK8s.NewSimpleClientset(),
				}, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(DeleteError),
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			testLog.Info("Testcase being executed", "testcase", testcase.testCaseName)

			scheme := setupScheme()
			client := testcase.clientFunc(testcase.objects)
			kubeClient = testcase.kubeClientFunc

			recoverStaleVolumeReconciler := &RecoverStaleVolumeReconciler{
				Client: client,
				Scheme: scheme,
				IsTest: true,
			}

			res, err := recoverStaleVolumeReconciler.Reconcile(testCtx, recoverStaleVolumeReconcileRequest)
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

func TestRecoverStaleVolumeSetupWithManager(t *testing.T) {
	t.Run("Positive: Successful", func(t *testing.T) {
		recoverStaleVolumeReconciler := &RecoverStaleVolumeReconciler{}
		recoverStaleVolumeReconciler.SetupWithManager(nil)
	})
}

func TestContains(t *testing.T) {
	t.Run("Positive: Successful", func(t *testing.T) {
		res := contains([]string{"ele1", "ele2"}, "ele1")
		assert.True(t, res)
	})
}

func TestCreateK8sClient(t *testing.T) {
	t.Run("", func(t *testing.T) {
		os.Setenv("KUBERNETES_SERVICE_HOST", "test-service-host")
		os.Setenv("KUBERNETES_SERVICE_PORT", "test-service-port")

		client, err := createK8sClient()
		assert.Nil(t, client)
		assert.Error(t, err)
	})
}
