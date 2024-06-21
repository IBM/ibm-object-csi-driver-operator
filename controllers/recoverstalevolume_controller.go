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

// Package controllers ...
package controllers

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	"github.com/go-logr/logr"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RecoverStaleVolumeReconciler reconciles a RecoverStaleVolume object
type RecoverStaleVolumeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	IsTest bool
}

// KubernetesClient ...
type KubernetesClient struct {
	Clientset kubernetes.Interface
}

var staleVolLog = logf.Log.WithName("recoverstalevolume_controller")
var kubeClient = createK8sClient

//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=recoverstalevolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=recoverstalevolumes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=recoverstalevolumes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RecoverStaleVolume object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *RecoverStaleVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := staleVolLog.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling RecoverStaleVolume")

	// Fetch RecoverStaleVolume instance
	instance := &objectdriverv1alpha1.RecoverStaleVolume{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8serr.IsNotFound(err) {
			reqLogger.Info("RecoverStaleVolume resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object.
		reqLogger.Error(err, "failed to get RecoverStaleVolume resource")
		return ctrl.Result{}, err
	}

	var logTailLines = instance.Spec.LogHistory
	if logTailLines == 0 {
		logTailLines = int64(constants.DefaultLogTailLines)
	}
	reqLogger.Info("Tail Log Lines to fetch", "number", logTailLines)

	for _, data := range instance.Spec.Data {
		namespace := data.Namespace
		deployments := util.Remove(data.Deployments, "")
		// If namespace is not set, use `default` ns
		if namespace == "" {
			namespace = constants.DefaultNamespace
		}
		reqLogger.Info("Data Requested", "namespace", namespace, "deployments", deployments)

		k8sOps := &crutils.K8sResourceOps{
			Client:    r.Client,
			Ctx:       ctx,
			Namespace: namespace,
		}

		// If applications are not set, then fetch all deployments from given ns
		if len(deployments) == 0 {
			deploymentsList, err := k8sOps.ListDeployment()
			if err != nil {
				reqLogger.Error(err, "failed to get deployment list")
				return ctrl.Result{}, err
			}

			depNames := []string{}
			for _, dep := range deploymentsList.Items {
				depNames = append(depNames, dep.Name)
			}
			deployments = depNames
		}

		pvcAndPVNamesMap, err := fetchCSIPVCAndPVNames(k8sOps, reqLogger)
		if err != nil {
			return ctrl.Result{}, err
		}
		reqLogger.Info("PVCs using CSI StorageClasses", "pvc:pv-names", pvcAndPVNamesMap)

		pvcNames := maps.Keys(pvcAndPVNamesMap)

		csiApplications, err := fetchDeploymentsUsingCSIVolumes(k8sOps, reqLogger, deployments, pvcNames)
		if err != nil {
			return ctrl.Result{}, err
		}
		reqLogger.Info("Deployments which are using CSI Volumes in given namespace", "namespace", namespace,
			"applications", csiApplications)

		if len(csiApplications) == 0 {
			reqLogger.Info("No Deployment found in requested namespace")
			continue
		}

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

		// Fetch all Pods in given namespace
		podsList, err := k8sOps.ListPod()
		if err != nil {
			reqLogger.Error(err, "failed to get pods list")
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched pods in given namespace", "namespace", namespace, "number-of-pods",
			len(podsList.Items))

		for ind, podData := range podsList.Items {
			podName := podData.Name
			if util.MatchesPrefix(csiApplications, podName) {
				reqLogger.Info("Application Pod found", "name", podName)

				volumesUsedByPod := podData.Spec.Volumes
				for _, volume := range volumesUsedByPod {
					pvcDetails := volume.VolumeSource.PersistentVolumeClaim
					if pvcDetails != nil {
						pvcName := pvcDetails.ClaimName

						if util.Contains(pvcNames, pvcName) {
							reqLogger.Info("Pod details", "pod-name", podName, "pvc-name", pvcName)
							nodeName := podData.Spec.NodeName
							volumeData, ok := nodeVolumePodMapping[nodeName]
							if !ok {
								volumeData = map[string][]string{}
							}

							volumeName := pvcAndPVNamesMap[pvcName]
							podData, ok := volumeData[volumeName]
							if !ok {
								podData = []string{}
							}

							if !util.Contains(podData, podName) {
								podData = append(podData, podName)
							}

							volumeData[volumeName] = podData
							nodeVolumePodMapping[nodeName] = volumeData

							deploymentPods[podName] = podsList.Items[ind]
						}
					}
				}
			}
		}
		reqLogger.Info("node-names maped with volumes and deployment pods", "nodeVolumeMap", nodeVolumePodMapping)

		// Get Pods in ibm-object-csi-driver-operator ns
		k8sOps.Namespace = constants.CSIOperatorNamespace
		podsInOpNs, err := k8sOps.ListPod()
		if err != nil {
			reqLogger.Error(err, "failed to fetch pods in namespace: "+constants.CSIOperatorNamespace)
			return ctrl.Result{}, err
		}
		reqLogger.Info("Successfully fetched pods in namespace", "number-of-pods", len(podsInOpNs.Items))

		for ind := range podsInOpNs.Items {
			pod := podsInOpNs.Items[ind]
			if strings.HasPrefix(pod.Name, constants.GetResourceName(constants.CSINode)) {
				csiNodeServerPods[pod.Spec.NodeName] = pod.Name
			}
		}
		reqLogger.Info("node-names maped with csi node-server pods", "csiNodeServerPods", csiNodeServerPods)

		// If CSI Driver Node Pods not found, reconcile
		if len(csiNodeServerPods) == 0 {
			continue
		}

		for nodeName, volumesData := range nodeVolumePodMapping {
			// Fetch volume stats from Logs of the Node Server Pod
			getVolStatsFromLogs, err := fetchVolumeStatsFromNodeServerPodLogs(ctx, csiNodeServerPods[nodeName],
				constants.CSIOperatorNamespace, logTailLines, r.IsTest)
			if err != nil {
				return ctrl.Result{}, err
			}
			reqLogger.Info("Volume Stats from NodeServer Pod Logs", "volume-stas", getVolStatsFromLogs)

			for volume, podData := range volumesData {
				getVolStatsFromLogs, ok := getVolStatsFromLogs[volume]
				if !ok {
					continue
				}

				if strings.Contains(getVolStatsFromLogs, constants.TransportEndpointError) {
					reqLogger.Info("Stale Volume Found", "volume", volume)

					for _, podName := range podData {
						reqLogger.Info("Pod using stale volume", "volume-name", volume, "pod-name", podName)

						pod := deploymentPods[podName]
						err = k8sOps.DeletePod(&pod)
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
				}
			}
		}
	}

	return ctrl.Result{RequeueAfter: constants.ReconcilationTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RecoverStaleVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectdriverv1alpha1.RecoverStaleVolume{}).
		Complete(r)
}

func fetchCSIPVCAndPVNames(k8sOps *crutils.K8sResourceOps, log logr.Logger) (map[string]string, error) {
	defer LogFunctionDuration(log, "fetchCSIPVCAndPVNames", time.Now())

	pvcList, err := k8sOps.ListPVC()
	if err != nil {
		log.Error(err, "failed to get pvc list")
		return nil, err
	}

	reqData := map[string]string{}

	for _, pvc := range pvcList.Items {
		log.Info("PVC Found", "pvc-name", pvc.Name, "namespace", pvc.Namespace)

		storageClassName := pvc.Spec.StorageClassName
		if storageClassName == nil {
			log.Info("PVC does not have any storageClass", "pvc-name", pvc.Name)
			continue
		}

		scName := *storageClassName
		if scName == "" {
			log.Info("PVC does not have any storageClass", "pvc-name", pvc.Name)
			continue
		}

		if strings.HasPrefix(scName, constants.StorageClassPrefix) && strings.HasSuffix(scName, constants.StorageClassSuffix) {
			reqData[pvc.Name] = pvc.Spec.VolumeName
		}
	}

	return reqData, nil
}

func fetchDeploymentsUsingCSIVolumes(k8sOps *crutils.K8sResourceOps, log logr.Logger, depNames []string,
	reqPVCNames []string) ([]string, error) {
	defer LogFunctionDuration(log, "fetchDeploymentsUsingCSIVolumes", time.Now())

	var reqDeploymentNames []string

	for _, name := range depNames {
		deployment, err := k8sOps.GetDeployment(name)
		if err != nil {
			if k8serr.IsNotFound(err) {
				log.Info("Deployment not found.")
				continue
			}
			log.Error(err, "failed to get deployment")
			return nil, err
		}

		volumes := deployment.Spec.Template.Spec.Volumes
		for _, vol := range volumes {
			pvcDetails := vol.VolumeSource.PersistentVolumeClaim
			if pvcDetails != nil {
				pvcName := pvcDetails.ClaimName
				if util.Contains(reqPVCNames, pvcName) {
					reqDeploymentNames = append(reqDeploymentNames, name)
					break
				}
			}
		}
	}

	return reqDeploymentNames, nil
}

func createK8sClient() (*KubernetesClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubernetesClient{
		Clientset: clientset,
	}, nil
}

func fetchVolumeStatsFromNodeServerPodLogs(ctx context.Context, nodeServerPod, namespace string, logTailLines int64,
	isTest bool) (map[string]string, error) {
	defer LogFunctionDuration(staleVolLog, "fetchVolumeStatsFromNodeServerPodLogs", time.Now())

	staleVolLog.Info("Input Parameters: ", "nodeServerPod", nodeServerPod, "namespace", namespace, "isTest", isTest)
	podLogOpts := &corev1.PodLogOptions{
		Container: constants.NodeContainerName,
		TailLines: &logTailLines,
	}

	k8sClient, err := kubeClient()
	if err != nil {
		return nil, err
	}
	request := k8sClient.Clientset.CoreV1().Pods(namespace).GetLogs(nodeServerPod, podLogOpts)

	nodePodLogs, err := request.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer nodePodLogs.Close() // #nosec G307 Close Stream

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, nodePodLogs)
	if err != nil {
		return nil, err
	}
	nodeServerPodLogs := buf.String()

	if isTest {
		nodeServerPodLogs = testNodeServerPodLogs
	}

	return parseLogs(nodeServerPodLogs), nil
}

// LogFunctionDuration calculates time taken by a method
func LogFunctionDuration(logger logr.Logger, methodName string, start time.Time) {
	duration := time.Since(start)
	logger.Info("Time to complete", methodName, duration.Seconds())
}
