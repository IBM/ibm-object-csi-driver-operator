package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	fakeget "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get"
	fakegetdeploy "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get/deployment"
	fakegetpvc "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_get/pvc"
	fakelist "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_list"
	fakelistnodeserverpod "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/fake/client_list/nodeserverpod"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	recoverstalevolumeTestLog = log.Log.WithName("recoverstalevolume_controller_test")

	recoverStaleVolCRName      = "test-vol-cr"
	recoverStaleVolCRNamespace = "test-namespace"
	testDeploymentName         = "test-deployment"
	testDeploymentNamespace    = "default"
	testPVName                 = "test-pv"
	testPVCName                = "test-pvc"
	testStorageClassName       = "test-csi-storage-class"

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

	deploymentPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName + "-pod1",
			Namespace: testDeploymentNamespace,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: testPVName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: testPVCName,
						},
					},
				},
			},
		},
	}

	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPVCName,
			Namespace: testDeploymentNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &testStorageClassName,
		},
	}
)

func TestRecoverStaleVolumeReconcile(t *testing.T) {
	testCases := []struct {
		testCaseName string
		objects      []runtime.Object
		clientFunc   func(objs []runtime.Object) client.WithWatch
		expectedResp reconcile.Result
		expectedErr  error
	}{
		// {
		// 	testCaseName: "Positive: Successful",
		// 	objects: []runtime.Object{
		// 		recoverStaleVolumeCR,
		// 		deployment,
		// 		deploymentPod,
		// 		pvc,
		// 	},
		// 	clientFunc: func(objs []runtime.Object) client.WithWatch {
		// 		return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
		// 	},
		// 	expectedResp: reconcile.Result{RequeueAfter: reconcileTime},
		// 	expectedErr:  nil,
		// },
		{
			testCaseName: "Positive: RecoverStaleVolume CR not found",
			objects:      []runtime.Object{},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
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
			expectedResp: reconcile.Result{RequeueAfter: reconcileTime},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to get RecoverStaleVolume CR",
			objects:      []runtime.Object{},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakeget.NewClientBuilder().WithRuntimeObjects(objs...).Build()
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
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
		{
			testCaseName: "Megative: Failed to get pvc",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakegetpvc.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(GetError),
		},
		{
			testCaseName: "Negative: Failed to get node-server-pods list",
			objects: []runtime.Object{
				recoverStaleVolumeCR,
				deployment,
				deploymentPod,
				pvc,
			},
			clientFunc: func(objs []runtime.Object) client.WithWatch {
				return fakelistnodeserverpod.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			},
			expectedResp: reconcile.Result{},
			expectedErr:  errors.New(ListError),
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			recoverstalevolumeTestLog.Info("Testcase being executed", "testcase", testcase.testCaseName)

			scheme := setupScheme()
			client := testcase.clientFunc(testcase.objects)

			recoverStaleVolumeReconciler := &RecoverStaleVolumeReconciler{
				Client: client,
				Scheme: scheme,
			}

			res, err := recoverStaleVolumeReconciler.Reconcile(context.TODO(), recoverStaleVolumeReconcileRequest)
			recoverstalevolumeTestLog.Info("Testcase return values", "result", res, "error", err)

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
