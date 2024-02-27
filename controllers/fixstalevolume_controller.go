/*
Copyright 2024.

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
var csiOperatorNamespace = "ibm-object-csi-operator-system"
var transportEndpointError = "transport endpoint is not connected"

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

	var logTailLines = instance.Spec.NoOfLogLines
	if logTailLines == 0 {
		logTailLines = int64(300)
	}
	reqLogger.Info("Tail Log Lines to fetch", "number", logTailLines)

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
				continue
			}
			reqLogger.Error(err, "failed to get deployment")
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched workload", "deployment", deployment)

		var csiNodeServerPods = map[string]string{} // {nodeName1: csiNodePod1, nodeName2: csiNodePod2, ...}
		var nodeVolumePodMapping = map[string]map[string][]string{}
		/*
			{
				nodeName1: {
					vol1: [pod1, pod2],
					vol2: [pod1]
				}
				nodeName2: {
					vol1: [pod3]
				}
			}
		*/
		var deploymentPods = map[string]corev1.Pod{} // {pod1: corev1.Pod{}, pod2: corev1.Pod}

		// Fetch Pods in given namespace
		var listOptions = &client.ListOptions{Namespace: deploymentNamespace}
		podsList := &corev1.PodList{}
		err = r.List(ctx, podsList, listOptions)
		if err != nil {
			reqLogger.Error(err, "failed to get pods list")
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched pods in workload namespace", "number-of-pods", len(podsList.Items))

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
								continue
							}
							reqLogger.Error(err, "failed to get pvc")
							return ctrl.Result{}, err
						}

						scName := *pvc.Spec.StorageClassName
						reqLogger.Info("PVC using Storage-class", "pvc-name", pvcName, "sc-name", scName)
						// Check if the volume is using csi storage-class
						if strings.Contains(scName, "csi") {

							nodeName := appPod.Spec.NodeName
							volumeData, ok := nodeVolumePodMapping[nodeName]
							if !ok {
								volumeData = map[string][]string{}
							}

							volumeName := pvc.Spec.VolumeName
							podData, ok := volumeData[volumeName]
							if !ok {
								podData = []string{}
							}

							if !contains(podData, appPod.Name) {
								podData = append(podData, appPod.Name)
							}

							volumeData[volumeName] = podData
							nodeVolumePodMapping[nodeName] = volumeData
							deploymentPods[appPod.Name] = appPod
						}
					}
				}
			}
		}
		reqLogger.Info("node-names maped with volumes and deployment pods", "nodeVolumeMap", nodeVolumePodMapping)

		// Get Pods in csiOperatorNamespace ns
		var listOptions2 = &client.ListOptions{Namespace: csiOperatorNamespace}
		csiPodsList := &corev1.PodList{}
		err = r.List(ctx, csiPodsList, listOptions2)
		if err != nil {
			reqLogger.Error(err, "failed to fetch csi pods")
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched pods in csi-plugin-operator ns", "number-of-pods", len(csiPodsList.Items))

		for ind := range csiPodsList.Items {
			pod := csiPodsList.Items[ind]
			if strings.HasPrefix(pod.Name, "ibm-object-csi-node") {
				reqLogger.Info("NodeServer Pod", "name", pod.Name)
				csiNodeServerPods[pod.Spec.NodeName] = pod.Name
			}
		}
		reqLogger.Info("node-names maped with node-server pods", "csiNodeServerPods", csiNodeServerPods)

		// If CSI Driver Node Pods not found, reconcile
		if len(csiNodeServerPods) == 0 {
			return ctrl.Result{RequeueAfter: reconcileTime}, nil
		}

		for nodeName, volumesData := range nodeVolumePodMapping {
			// Fetch volume stats from Logs of the Node Server Pod
			getVolStatsFromLogs, err := fetchVolumeStatsFromNodeServerLogs(ctx, csiNodeServerPods[nodeName], logTailLines)
			if err != nil {
				return ctrl.Result{}, err
			}
			reqLogger.Info("Volume Stats from NodeServer Pod Logs", "volume-stas", getVolStatsFromLogs)

			for volume, podData := range volumesData {
				volStats, ok := getVolStatsFromLogs[volume]
				if !ok {
					continue
				}

				if strings.Contains(volStats, transportEndpointError) {
					reqLogger.Info("Stale Volume Found", "volume", volume)

					reqLogger.Info("Restarting Pods!!")
					for _, podName := range podData {
						reqLogger.Info("Pod using stale volume", "volume-name", volume, "pod-name", podName)

						var zero int64 = 0
						var deleteOptions = &client.DeleteOptions{GracePeriodSeconds: &zero}

						pod := deploymentPods[podName]
						err = r.Delete(ctx, &pod, deleteOptions)
						if err != nil {
							if k8serr.IsNotFound(err) {
								reqLogger.Info("Pod not found.")
								continue
							}
							reqLogger.Error(err, "failed to delete pod")
							return ctrl.Result{}, err
						}
						reqLogger.Info("Pod deleted.")
					}
					reqLogger.Info("Pods Restarted!!")
				}
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

func fetchVolumeStatsFromNodeServerLogs(ctx context.Context, nodeServerPod string, logTailLines int64) (map[string]string, error) {
	podLogOpts := &corev1.PodLogOptions{
		Container: "ibm-object-csi-node",
		TailLines: &logTailLines,
	}

	k8sClient, err := createK8sClient()
	if err != nil {
		return nil, err
	}
	request := k8sClient.CoreV1().Pods(csiOperatorNamespace).GetLogs(nodeServerPod, podLogOpts)

	nodePodLogs, err := request.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer nodePodLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, nodePodLogs)
	if err != nil {
		return nil, err
	}
	nodeServerPodLogs := buf.String()

	return parseLogs(nodeServerPodLogs), nil
}
