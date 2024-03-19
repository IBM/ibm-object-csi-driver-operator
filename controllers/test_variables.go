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

var testNodeServerPodLogs = `I0207 09:02:14.466230       1 nodeserver.go:188] NodeGetVolumeStats: Request: {VolumeId:test-pv-1 VolumePath:/var/data/kubelet/pods/360289c4-6eca-4275-a9ce-14938335117e/volumes/kubernetes.io~csi/pvc-7400b05a-12a4-43ac-bfda-a5ce6445b6ef/mount StagingTargetPath: XXX_NoUnkeyedLiteral:{} XXX_unrecognized:[] XXX_sizecache:0}
I0207 09:02:14.466270       1 nodeserver.go:198] NodeGetVolumeStats: Start getting Stats
E0207 09:02:14.466335       1 server.go:158] GRPC error: transport endpoint is not connected`

func setupScheme() *runtime.Scheme {
	s := scheme.Scheme
	_ = v1alpha1.AddToScheme(s)
	return s
}
