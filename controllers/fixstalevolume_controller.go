/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	csiv1alpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
)

// FixStaleVolumeReconciler reconciles a FixStaleVolume object
type FixStaleVolumeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var staleVolLog = logf.Log.WithName("fixstalevolume_controller")

//+kubebuilder:rbac:groups=csi.ibm.com,resources=fixstalevolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ibm.com,resources=fixstalevolumes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ibm.com,resources=fixstalevolumes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FixStaleVolume object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *FixStaleVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := staleVolLog.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling FixStaleVolume")

	// Fetch FixStaleVolume instance
	instance := &csiv1alpha1.FixStaleVolume{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8serr.IsNotFound(err) {
			reqLogger.Info("FixStaleVolume resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object.
		reqLogger.Error(err, "failed to get FixStaleVolume resource")
		return ctrl.Result{}, err
	}

	deploymentName := instance.Spec.DeploymentName
	deploymentNamespace := instance.Spec.DeploymentNamespace
	reqLogger.Info("Requested Workload to watch", "deployment-name", deploymentName, "deployment-namespace", deploymentNamespace)

	// If workload not found, reconcile
	if deploymentName == "" {
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	if deploymentNamespace == "" {
		deploymentNamespace = "default"
	}

	// Fetch deployment
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: deploymentNamespace}, deployment)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	reqLogger.Info("Successfully fetched deployment", "deployment", deployment)

	// Fetch Deployment Pods
	var listOptions = &client.ListOptions{Namespace: deploymentNamespace}
	podsList := &corev1.PodList{}
	err = r.List(ctx, podsList, listOptions)
	if err != nil {
		return ctrl.Result{}, err
	}
	reqLogger.Info("Successfully fetched deployment pods", "number-of-pods", len(podsList.Items))

	nodeNameAndVolMap := map[string][]string{}
	volAndDeployPodsMap := map[string][]string{}
	for ind := range podsList.Items {
		pod := podsList.Items[ind]
		reqLogger.Info("Pod", "name", pod.Name)

		if strings.Contains(pod.Name, deploymentName) {

			volumesUsed := pod.Spec.Volumes
			for _, volume := range volumesUsed {

				pvcDetails := volume.VolumeSource.PersistentVolumeClaim
				if pvcDetails != nil {
					pvcName := pvcDetails.ClaimName
					reqLogger.Info("PVC", "name", pvcName)
					pvc := &corev1.PersistentVolumeClaim{}
					err = r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: "default"}, pvc)
					if err != nil {
						if k8serr.IsNotFound(err) {
							reqLogger.Info("PVC not found", "name", pvcName)
							continue
						}
						return ctrl.Result{}, err
					}

					// If the volume is using csi storage-class, add in the CSIVolumesUsed slice
					reqLogger.Info("PVC using Storage-class", "name", *pvc.Spec.StorageClassName)
					if strings.Contains(*pvc.Spec.StorageClassName, "csi") {
						reqLogger.Info("Add PVC in nodeNameAndVolMap map", "name", pvcName)
						nodeNameAndVolMap[pod.Spec.NodeName] = append(nodeNameAndVolMap[pod.Spec.NodeName], pvc.Spec.VolumeName) //check for duplicates
						volAndDeployPodsMap[pvc.Spec.VolumeName] = append(volAndDeployPodsMap[pvc.Spec.VolumeName], pod.Name)    //check for duplicates
					}
				}
			}
		}
	}
	reqLogger.Info("node-names maped with volumes mounted", "nodeNameAndVolMap", nodeNameAndVolMap)

	// If there is no csi-volume used
	if len(nodeNameAndVolMap) == 0 {
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Get CSI Node Pods
	var listOptions2 = &client.ListOptions{Namespace: "ibm-object-csi-operator-system"}
	csiPodsList := &corev1.PodList{}
	err = r.List(ctx, csiPodsList, listOptions2)
	if err != nil {
		return ctrl.Result{}, err
	}
	reqLogger.Info("cos-csi pods", "number", len(csiPodsList.Items))

	nodeNameAndNodePodMap := map[string]corev1.Pod{}
	for ind := range csiPodsList.Items {
		pod := csiPodsList.Items[ind]
		if strings.HasPrefix(pod.Name, "ibm-object-csi-node") {
			nodeNameAndNodePodMap[pod.Spec.NodeName] = pod
		}
	}
	reqLogger.Info("node-names maped with node server pods", "length of nodeNameAndNodePodMap", len(nodeNameAndNodePodMap))

	// Fetch Logs of the Node Server Pod having stale volumes mounted
	// for nodeName, volList := range nodeNameAndVolMap {
	for nodeName, volumes := range nodeNameAndVolMap {
		nodePod, ok := nodeNameAndNodePodMap[nodeName]
		if !ok {
			return ctrl.Result{}, err
		}

		config, err := rest.InClusterConfig()
		if err != nil {
			return ctrl.Result{}, err
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return ctrl.Result{}, err
		}

		tailLines := int64(300)

		podLogOpts := &corev1.PodLogOptions{
			Container: "ibm-object-csi-node",
			TailLines: &tailLines,
		}
		req := clientset.CoreV1().Pods("ibm-object-csi-operator-system").GetLogs(nodePod.Name, podLogOpts)

		nodePodLogs, err := req.Stream(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		defer nodePodLogs.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, nodePodLogs)
		if err != nil {
			return ctrl.Result{}, err
		}
		logs := buf.String()

		getVolStats := parseLogs(logs)

		fmt.Println(getVolStats)

		for _, vol := range volumes {
			stats, ok := getVolStats[vol]
			if !ok {
				continue
			}

			if strings.Contains(stats, "transport endpoint is not connected") {
				fmt.Println("Stale Volume Found", vol)
				// restart the pod
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FixStaleVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1alpha1.FixStaleVolume{}).
		Complete(r)
}
