package controllers

import (
	"errors"
	"os"
	"testing"

	"github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	fakedelete "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_delete"
	fakeget "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_get"
	fakegetdeploy "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_get/deployment"
	fakelist "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_list"
	fakelistdeploypod "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_list/deploymentpod"
	fakelistnodeserverpod "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_list/nodeserverpod"
	fakelistpvc "github.com/IBM/ibm-object-csi-driver-operator/controllers/fake/client_list/pvc"
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
	emptyVal = ""

	recoverStaleVolumeReconcileRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      recoverStaleVolCRName,
			Namespace: TestNamespace,
		},
	}

	recoverStaleVolumeCR = &v1alpha1.RecoverStaleVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoverStaleVolCRName,
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.RecoverStaleVolumeSpec{
			LogHistory: int64(100),
			Data: []v1alpha1.NamespacedDeploymentData{
				{
					Namespace: testDeploymentNamespace,
				},
			},
		},
	}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName,
			Namespace: testDeploymentNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: testPVCName1,
								},
							},
						},
					},
				},
			},
		},
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

	nodeServerPod1 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GetResourceName(constants.CSINode) + "-pod1",
			Namespace: constants.CSIOperatorNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode1,
		},
	}

	nodeServerPod2 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GetResourceName(constants.CSINode) + "-pod2",
			Namespace: constants.CSIOperatorNamespace,
		},
		Spec: corev1.PodSpec{
			NodeName: testNode2,
		},
	}

	nodeServerPod3 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GetResourceName(constants.CSINode) + "-pod3",
			Namespace: constants.CSIOperatorNamespace,
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
				pvc1,
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
			expectedResp: reconcile.Result{RequeueAfter: constants.ReconcilationTime},
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
			testCaseName: "Incomplete: StorageClassName is nil",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				&corev1.PersistentVolumeClaim{
					ObjectMeta: pvc1.ObjectMeta,
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: nil,
					},
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: constants.ReconcilationTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Incomplete: StorageClassName is empty",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				&corev1.PersistentVolumeClaim{
					ObjectMeta: pvc1.ObjectMeta,
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &emptyVal,
					},
				},
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: constants.ReconcilationTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Incomplete: No deploymnets to watch",
			objects: []runtime.Object{
				&v1alpha1.RecoverStaleVolume{
					ObjectMeta: recoverStaleVolumeCR.ObjectMeta,
					Spec: v1alpha1.RecoverStaleVolumeSpec{
						Data: []v1alpha1.NamespacedDeploymentData{
							{
								Namespace:   testDeploymentNamespace,
								Deployments: []string{testDeploymentName},
							},
						},
					},
				},
				pvc1,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{RequeueAfter: constants.ReconcilationTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Incomplete: Node Server Pods not found",
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
			expectedResp: reconcile.Result{RequeueAfter: constants.ReconcilationTime},
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
			testCaseName: "Negative: Failed to list deployments",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
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
			testCaseName: "Negative: Failed to list pvc",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakelistpvc.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
		{
			testCaseName: "Negative: Failed to get deployment",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				pvc1,
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
			testCaseName: "Negative: Failed to list deployment pods",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod1,
				pvc1,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakelistdeploypod.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			kubeClientFunc: func() (*KubernetesClient, error) {
				return nil, nil
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
		{
			testCaseName: "Negative: Failed to list node-server-pods",
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
			TestLog.Info("Testcase being executed", "testcase", testcase.testCaseName)

			scheme := setupScheme()
			client := testcase.clientFunc(testcase.objects)
			kubeClient = testcase.kubeClientFunc

			recoverStaleVolumeReconciler := &RecoverStaleVolumeReconciler{
				Client: client,
				Scheme: scheme,
				IsTest: true,
			}

			res, err := recoverStaleVolumeReconciler.Reconcile(TestCtx, recoverStaleVolumeReconcileRequest)
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

func TestRecoverStaleVolumeSetupWithManager(t *testing.T) {
	t.Run("Positive: Successful", func(t *testing.T) {
		recoverStaleVolumeReconciler := &RecoverStaleVolumeReconciler{}
		if err := recoverStaleVolumeReconciler.SetupWithManager(nil); err != nil {
			TestLog.Error(err, "failed to setup controller with nanager")
		}
	})
}

func TestCreateK8sClient(t *testing.T) {
	t.Run("", func(t *testing.T) {
		_ = os.Setenv("KUBERNETES_SERVICE_HOST", "test-service-host") // #nosec G104 Skip error
		_ = os.Setenv("KUBERNETES_SERVICE_PORT", "test-service-port") // #nosec G104 Skip error

		client, err := createK8sClient()
		assert.Nil(t, client)
		assert.Error(t, err)
	})
}
