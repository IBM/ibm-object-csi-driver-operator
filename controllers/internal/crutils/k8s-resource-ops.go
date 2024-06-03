package crutils

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sResourceOps struct {
	client.Client
	Ctx       context.Context
	Namespace string
}

func (op *K8sResourceOps) GetDeployment(name string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := op.Get(op.Ctx, types.NamespacedName{Name: name, Namespace: op.Namespace}, deployment)
	return deployment, err
}

func (op *K8sResourceOps) ListDeployment() (*appsv1.DeploymentList, error) {
	var listOptions = &client.ListOptions{Namespace: op.Namespace}
	deploymentList := &appsv1.DeploymentList{}
	err := op.List(op.Ctx, deploymentList, listOptions)
	return deploymentList, err
}

func (op *K8sResourceOps) DeletePod(pod *corev1.Pod) error {
	var zero int64
	var deleteOptions = &client.DeleteOptions{GracePeriodSeconds: &zero}
	err := op.Delete(op.Ctx, pod, deleteOptions)
	return err
}

func (op *K8sResourceOps) ListPod() (*corev1.PodList, error) {
	var listOptions = &client.ListOptions{Namespace: op.Namespace}
	podList := &corev1.PodList{}
	err := op.List(op.Ctx, podList, listOptions)
	return podList, err
}

func (op *K8sResourceOps) GetPVC(name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := op.Get(op.Ctx, types.NamespacedName{Name: name, Namespace: op.Namespace}, pvc)
	return pvc, err
}

func (op *K8sResourceOps) ListPVC() (*corev1.PersistentVolumeClaimList, error) {
	var listOptions = &client.ListOptions{Namespace: op.Namespace}
	pvcList := &corev1.PersistentVolumeClaimList{}
	err := op.List(op.Ctx, pvcList, listOptions)
	return pvcList, err
}
