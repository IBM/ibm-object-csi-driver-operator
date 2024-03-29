package controllers

import (
	"context"

	"github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	CreateError = "failed to create"
	DeleteError = "failed to delete"
	GetError    = "failed to get"
	ListError   = "failed to list"
	UpdateError = "failed to update"

	NotFoundError = "not found"
)

var (
	testLog = log.Log.WithName("test")
	testCtx = context.TODO()

	currentTime = metav1.Now()

	ibmObjectCSICRName      = "test-csi-cr"
	ibmObjectCSICRNamespace = "test-namespace"
	ibmObjectCSIfinalizer   = "ibmobjectcsi.objectdriver.csi.ibm.com"

	recoverStaleVolCRName      = "test-vol-cr"
	recoverStaleVolCRNamespace = "test-namespace"
	testDeploymentName         = "test-deployment"
	testDeploymentNamespace    = "default"
	testPVName1                = "test-pv-1"
	testPVName2                = "test-pv-2"
	testPVCName1               = "test-pvc-1"
	testPVCName2               = "test-pvc-2"
	testStorageClassName       = "test-csi-storage-class"
	testNode1                  = "test-node-1"
	testNode2                  = "test-node-2"
	testNode3                  = "test-node-3"
)

var testNodeServerPodLogs = `E0319 05:32:00.429871       1 nodeserver.go:245] NodeGetVolumeStats: error occurred while getting volume stats map[Error:transport endpoint is not connected VolumeId:test-pv-1]`

func setupScheme() *runtime.Scheme {
	s := scheme.Scheme
	_ = v1alpha1.AddToScheme(s)
	return s
}
