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
var reconcileTime = 2 * time.Minute
var logTailLines = int64(300)

type RequiredData struct {
	NodeName       string
	NodeServerPod  string
	DeploymentPods []string
	VolumesInUse   []string
}

// nodeName: {
// 	vol1:[pod1,pod2]
// 	vol2:[pod1]
// 	vol3:[pod3]
// }

// {nodeName:nodeServerPod}

//+kubebuilder:rbac:groups=csi.ibm.com,resources=fixstalevolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ibm.com,resources=fixstalevolumes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ibm.com,resources=fixstalevolumes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=patch

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

	for _, data := range instance.Spec.Deployment {
		deploymentName := data.DeploymentName
		deploymentNamespace := data.DeploymentNamespace
		reqLogger.Info("Requested Workload to watch", "deployment-name", deploymentName, "deployment-namespace", deploymentNamespace)

		// If workload not found, reconcile
		if deploymentName == "" {
			return ctrl.Result{RequeueAfter: reconcileTime}, nil
		}

		// If namespace is not set, use `default` ns
		if deploymentNamespace == "" {
			deploymentNamespace = "default"
		}

		// Fetch deployment
		deployment := &appsv1.Deployment{}
		err = r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: deploymentNamespace}, deployment)
		if err != nil {
			if k8serr.IsNotFound(err) {
				reqLogger.Info("Deployment not found.")
				continue // ADD WHETHER TO RECONCILE OR IGNORE
			}
			reqLogger.Error(err, "failed to get deployment")
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched workload", "deployment", deployment)

		// Fetch Pods in given namespace
		var listOptions = &client.ListOptions{Namespace: deploymentNamespace}
		podsList := &corev1.PodList{}
		err = r.List(ctx, podsList, listOptions)
		if err != nil {
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched deployment pods", "number-of-pods", len(podsList.Items))

		requiredData := map[string]RequiredData{}

		for ind := range podsList.Items {
			appPod := podsList.Items[ind]
			if strings.Contains(appPod.Name, deploymentName) {
				reqLogger.Info("Deployment Pod", "name", appPod.Name)

				volumesUsed := appPod.Spec.Volumes
				for _, volume := range volumesUsed {
					pvcDetails := volume.VolumeSource.PersistentVolumeClaim
					if pvcDetails != nil {
						pvcName := pvcDetails.ClaimName
						reqLogger.Info("PVC In Use", "pod-name", appPod.Name, "pvc-name", pvcName)

						pvc := &corev1.PersistentVolumeClaim{}
						err = r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: deploymentNamespace}, pvc)
						if err != nil {
							if k8serr.IsNotFound(err) {
								reqLogger.Info("PVC not found.")
								continue // ADD WHETHER TO RECONCILE OR IGNORE
							}
							reqLogger.Error(err, "failed to get pvc")
							return ctrl.Result{}, err
						}

						scName := *pvc.Spec.StorageClassName
						reqLogger.Info("PVC using Storage-class", "pvc-name", pvcName, "sc-name", scName)
						// Check if the volume is using csi storage-class
						if strings.Contains(scName, "csi") {

							nodeName := appPod.Spec.NodeName
							val, ok := requiredData[nodeName]
							if !ok {
								requiredData[nodeName] = RequiredData{}
							}

							volumes := val.VolumesInUse
							if !contains(volumes, pvc.Spec.VolumeName) {
								volumes = append(volumes, pvc.Spec.VolumeName)
							}
							pods := val.DeploymentPods
							if !contains(pods, appPod.Name) {
								pods = append(pods, appPod.Name)
							}

							requiredData[nodeName] = RequiredData{
								NodeName:       nodeName,
								DeploymentPods: pods,
								VolumesInUse:   volumes,
							}
						}
					}
				}
			}
		}
		reqLogger.Info("node-names maped with user pods and volumes in use", "requiredData", requiredData)

		// If there is no csi-volume used, reconcile
		if len(requiredData) == 0 {
			return ctrl.Result{RequeueAfter: reconcileTime}, nil
		}

		// Get Pods in "ibm-object-csi-operator-system" ns
		var listOptions2 = &client.ListOptions{Namespace: "ibm-object-csi-operator-system"}
		csiPodsList := &corev1.PodList{}
		err = r.List(ctx, csiPodsList, listOptions2)
		if err != nil {
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched pods in ibm-object-csi-operator-system ns", "number-of-pods", len(csiPodsList.Items))

		for ind := range csiPodsList.Items {
			pod := csiPodsList.Items[ind]
			if strings.HasPrefix(pod.Name, "ibm-object-csi-node") {
				reqLogger.Info("NodeServer Pod", "name", pod.Name)

				nodeName := pod.Spec.NodeName
				val, ok := requiredData[nodeName]
				if !ok {
					continue
				}

				requiredData[nodeName] = RequiredData{
					NodeName:       nodeName,
					NodeServerPod:  pod.Name,
					DeploymentPods: val.DeploymentPods,
					VolumesInUse:   val.VolumesInUse,
				}
			}
		}
		reqLogger.Info("node-names maped with required data", "requiredData", requiredData)

		isPodsRestarted := false

		for _, data := range requiredData {
			// Fetch Logs of the Node Server Pod having stale volumes mounted
			podLogOpts := &corev1.PodLogOptions{
				Container: "ibm-object-csi-node",
				TailLines: &logTailLines,
			}

			k8sClient, err := createK8sClient()
			if err != nil {
				return ctrl.Result{}, err
			}
			request := k8sClient.CoreV1().Pods("ibm-object-csi-operator-system").GetLogs(data.NodeServerPod, podLogOpts)

			nodePodLogs, err := request.Stream(ctx)
			if err != nil {
				return ctrl.Result{}, err
			}
			defer nodePodLogs.Close()

			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, nodePodLogs)
			if err != nil {
				return ctrl.Result{}, err
			}
			nodeServerPodLogs := buf.String()

			getVolStatsFromLogs := parseLogs(nodeServerPodLogs)
			reqLogger.Info("Volume Stats from NodeServer Pod Logs", "volume-stas", getVolStatsFromLogs)

			for _, volumeInUse := range data.VolumesInUse {
				volStats, ok := getVolStatsFromLogs[volumeInUse]
				if !ok {
					continue
				}

				if strings.Contains(volStats, "transport endpoint is not connected") {
					reqLogger.Info("Stale Volume Found", "volume", volumeInUse)

					// reqLogger.Info("Restarting Deployment....")
					// deploymentAnnotations := deployment.GetAnnotations()
					// if deploymentAnnotations == nil {
					// 	deploymentAnnotations = map[string]string{}
					// }
					// deploymentAnnotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
					// deployment.SetAnnotations(deploymentAnnotations)
					// // err = r.Update(ctx, deployment)
					// err = r.Patch(ctx, deployment, client.MergeFrom(deployment))
					// if err != nil {
					// 	reqLogger.Error(err, "failed to restart deployment")
					// 	return ctrl.Result{}, err
					// }
					// reqLogger.Info("Deployment Restarted!!")

					reqLogger.Info("Restarting Pods....")
					for _, podName := range data.DeploymentPods {
						pod := &corev1.Pod{}
						err = r.Get(ctx, types.NamespacedName{Name: podName, Namespace: deploymentNamespace}, pod)
						if err != nil {
							if k8serr.IsNotFound(err) {
								reqLogger.Info("Pod not found.")
								continue
							}
							reqLogger.Error(err, "failed to get pod")
							return ctrl.Result{}, err
						}

						err = r.Delete(ctx, pod)
						if err != nil {
							if k8serr.IsNotFound(err) {
								reqLogger.Info("Pod not found.")
								continue
							}
							reqLogger.Error(err, "failed to delete pod")
							return ctrl.Result{}, err
						}
					}
					reqLogger.Info("Pods Restarted!!")
					isPodsRestarted = true
					break
				}

				if isPodsRestarted {
					break
				}
			}

			if isPodsRestarted {
				break
			}
		}
	}

	return ctrl.Result{RequeueAfter: reconcileTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FixStaleVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1alpha1.FixStaleVolume{}).
		Complete(r)
}

func contains(slice []string, value string) bool {
	for _, val := range slice {
		if val == value {
			return true
		}
	}
	return false
}

func createK8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
