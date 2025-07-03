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
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	crutils "github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	clustersyncer "github.com/IBM/ibm-object-csi-driver-operator/controllers/syncer"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util/common"
	"github.com/IBM/ibm-object-csi-driver-operator/version"
	"github.com/go-logr/logr"
	"github.com/presslabs/controller-util/pkg/syncer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler func(instance *crutils.IBMObjectCSI) error

var csiLog = logf.Log.WithName("ibmobjectcsi_controller")

// IBMObjectCSIReconciler reconciles a IBMObjectCSI object
type IBMObjectCSIReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	ControllerHelper *common.ControllerHelper
}

//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=ibmobjectcsis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=ibmobjectcsis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=ibmobjectcsis/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;delete;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;create;delete;list;watch;update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;delete;list;watch;update;create;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=*
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;statefulsets,verbs=get;list;watch;update;create;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;delete;get;watch;list
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=create;delete;get;watch;list;update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resourceNames=ibm-object-csi-operator,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=create;delete;get;watch;list
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csinodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=security.openshift.io,resourceNames=anyuid;privileged,resources=securitycontextconstraints,verbs=use
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=create;list;watch;delete
//+kubebuilder:rbac:groups=objectdriver.csi.ibm.com,resources=*,verbs=*
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=create;get;list;watch;delete;update
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IBMObjectCSI object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *IBMObjectCSIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	reqLogger := csiLog.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	r.ControllerHelper.Log = csiLog

	// Check if the reconcile was triggered by the ConfigMap events
	if req.Namespace == constants.CSIOperatorNamespace && req.Name == constants.ParamsConfigMap {
		reqLogger.Info("Reconcile triggered by create/update event on ConfigMap")
		// Handle the update of IBMObjectCSI
		return r.handleConfigMapReconcile(ctx, req)
	}
	reqLogger.Info("Reconciling IBMObjectCSI")

	// Fetch the CSIDriver instance
	instance := crutils.New(&objectdriverv1alpha1.IBMObjectCSI{})
	err := r.Get(ctx, req.NamespacedName, instance.Unwrap())
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	//setting default values for a Kubernetes Custom Resource using the Scheme's Default() method, based on the Go type definition of the Custom Resource.
	//This ensures that the CR has all the required fields with default values before further processing or reconciliation by the operator.
	r.Scheme.Default(instance.Unwrap())
	if err := r.ControllerHelper.AddFinalizerIfNotPresent(
		instance, instance.Unwrap()); err != nil {
		return reconcile.Result{}, err
	}

	s3Provider := instance.Spec.S3Provider
	if s3Provider == "" {
		s3Provider = constants.S3ProviderIBM
	}
	r.ControllerHelper.S3Provider = s3Provider
	r.ControllerHelper.S3ProviderRegion = instance.Spec.S3ProviderRegion

	// If the deletion timestamp is set, perform cleanup operations and remove a finalizer before returning from the reconciliation process.
	if !instance.GetDeletionTimestamp().IsZero() {
		if err := r.deleteClusterRoleBindings(instance); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.deleteClusterRoles(instance); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.deleteStorageClasses(instance); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.deleteCSIDriver(instance); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.ControllerHelper.RemoveFinalizer(
			instance, instance.Unwrap()); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	originalStatus := *instance.Status.DeepCopy()

	// Fetch the ConfigMap instance
	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: constants.ParamsConfigMap, Namespace: constants.CSIOperatorNamespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ConfigMap not found. Retry after 5 seconds...", "name", constants.ParamsConfigMap, "namespace", constants.CSIOperatorNamespace)
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		reqLogger.Error(err, "Failed to get ConfigMap", constants.ParamsConfigMap)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if crUpdateRequired := checkIfupdateCRFromConfigMapRequired(instance.Unwrap(), configMap); crUpdateRequired {
		// Update the instance in the Kubernetes API server
		reqLogger.Info("IBMObjectCSI spec is not in sync with configmap data. Updating IBMObjectCSI CR...")
		err = r.Update(ctx, instance.Unwrap())
		if err != nil {
			reqLogger.Error(err, "Failed to update IBMObjectCSI instance as per configMap data")
			return reconcile.Result{}, err
		}
		reqLogger.Info("IBMObjectCSI CR is updated as per configmap data")
		return reconcile.Result{}, nil
	}

	// create the resources if not exist
	for _, rec := range []reconciler{
		r.reconcileCSIDriver,
		r.reconcileServiceAccount,
		r.reconcileClusterRole,
		r.reconcileClusterRoleBinding,
	} {
		if err = rec(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// sync the resources which change over time
	csiControllerSyncer := clustersyncer.NewCSIControllerSyncer(r.Client, instance)
	if err := syncer.Sync(ctx, csiControllerSyncer, r.Recorder); err != nil {
		return reconcile.Result{}, err
	}

	csiNodeSyncer := clustersyncer.NewCSINodeSyncer(r.Client, instance)
	if err := syncer.Sync(ctx, csiNodeSyncer, r.Recorder); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.reconcileStorageClasses(instance); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatus(instance, originalStatus); err != nil {
		return reconcile.Result{}, err
	}

	// Resources created successfully - don't requeue
	return reconcile.Result{}, nil
}

// handleConfigMapReconcile handles reconciliation triggered by the ConfigMap event
func (r *IBMObjectCSIReconciler) handleConfigMapReconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	reqLogger := csiLog.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Handling Reconcile IBMObjectCSI for ConfigMap event")

	// Fetch the ConfigMap instance
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ConfigMap not found. Ignoring configmap event...", "name", req.Name, "namespace", req.Namespace)
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get ConfigMap", req.Name)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Fetch the IBMObjectCSI instance
	instance := &objectdriverv1alpha1.IBMObjectCSI{}

	err = r.Get(ctx, types.NamespacedName{
		Namespace: constants.CSIOperatorNamespace,
		Name:      constants.ObjectCSIDriver},
		instance)
	if err != nil {
		reqLogger.Error(err, "Failed to get IBMObjectCSI instance")
		return reconcile.Result{}, err
	}
	reqLogger.Info("IBMObjectCSI CR fetched successfully")

	if crUpdateRequired := checkIfupdateCRFromConfigMapRequired(instance, configMap); crUpdateRequired {
		// Update the instance in the Kubernetes API server
		reqLogger.Info("IBMObjectCSI spec is not in sync with configmap data. Updating IBMObjectCSI CR...")
		err = r.Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update IBMObjectCSI instance as per configMap data")
			return reconcile.Result{}, err
		}
		reqLogger.Info("IBMObjectCSI CR is updated as per configmap data")
		return reconcile.Result{}, nil
	}
	reqLogger.Info("IBMObjectCSI spec is already in sync with configmap data. IBMObjectCSI CR update is not needed")
	return reconcile.Result{}, nil
}

func (r *IBMObjectCSIReconciler) updateStatus(instance *crutils.IBMObjectCSI, originalStatus objectdriverv1alpha1.IBMObjectCSIStatus) error {
	logger := csiLog.WithName("updateStatus")

	controllerDeployment, err := r.getControllerDeployment(instance)
	if err != nil {
		return err
	}

	nodeDaemonSet, err := r.getNodeDaemonSet(instance)
	if err != nil {
		return err
	}

	instance.Status.ControllerReady = r.isControllerReady(controllerDeployment)
	instance.Status.NodeReady = r.isNodeReady(nodeDaemonSet)
	phase := objectdriverv1alpha1.DriverPhaseNone
	if instance.Status.ControllerReady && instance.Status.NodeReady {
		phase = objectdriverv1alpha1.DriverPhaseRunning
	} else {
		if !instance.Status.ControllerReady {
			controllerPod, err := r.getControllerPod(controllerDeployment)
			if err != nil {
				logger.Error(err, "failed to get controller pod")
				return err
			}

			if !r.areAllPodImagesSynced(controllerDeployment, controllerPod) {
				r.restartControllerPodfromDeployment(logger, controllerDeployment, controllerPod) // #nosec G104 Skip error
			}
		}
		phase = objectdriverv1alpha1.DriverPhaseCreating
	}
	instance.Status.Phase = phase
	instance.Status.Version = version.DriverVersion

	if !reflect.DeepEqual(originalStatus, instance.Status) {
		logger.Info("updating IBMObjectCSI status", "name", instance.Name, "from", originalStatus, "to", instance.Status)
		sErr := r.Status().Update(context.TODO(), instance.Unwrap())
		if sErr != nil {
			return sErr
		}
	}

	return nil
}

func (r *IBMObjectCSIReconciler) restartControllerPodfromDeployment(logger logr.Logger,
	controllerDeployment *appsv1.Deployment, controllerPod *corev1.Pod) error {
	logger.Info("controller requires restart",
		"ReadyReplicas", controllerDeployment.Status.ReadyReplicas,
		"Replicas", controllerDeployment.Status.Replicas)
	logger.Info("restarting csi controller")
	return r.Delete(context.TODO(), controllerPod)
}

func (r *IBMObjectCSIReconciler) getControllerPod(controllerDeployment *appsv1.Deployment) (*corev1.Pod, error) {
	var listOptions = &client.ListOptions{Namespace: controllerDeployment.Namespace}
	podsList := &corev1.PodList{}
	err := r.List(context.TODO(), podsList, listOptions)
	if err != nil {
		return nil, err
	}

	for _, pod := range podsList.Items {
		if strings.HasPrefix(pod.Name, controllerDeployment.Name) {
			return &pod, nil
		}
	}

	err = errors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "controller pod")
	return nil, err
}

func (r *IBMObjectCSIReconciler) areAllPodImagesSynced(controllerDeployment *appsv1.Deployment, controllerPod *corev1.Pod) bool {
	logger := csiLog.WithName("areAllPodImagesSynced")
	deploymentContainers := controllerDeployment.Spec.Template.Spec.Containers
	podContainers := controllerPod.Spec.Containers
	if len(deploymentContainers) != len(podContainers) {
		return false
	}
	for i := 0; i < len(deploymentContainers); i++ {
		deploymentImage := deploymentContainers[i].Image
		podImage := podContainers[i].Image

		if deploymentImage != podImage {
			logger.Info("csi controller image not in sync",
				"deploymentImage", deploymentImage, "podImage", podImage)
			return false
		}
	}
	return true
}

func (r *IBMObjectCSIReconciler) isControllerReady(controller *appsv1.Deployment) bool {
	return controller.Status.ReadyReplicas == controller.Status.Replicas
}

func (r *IBMObjectCSIReconciler) isNodeReady(node *appsv1.DaemonSet) bool {
	return node.Status.DesiredNumberScheduled == node.Status.NumberAvailable
}

func (r *IBMObjectCSIReconciler) getNodeDaemonSet(instance *crutils.IBMObjectCSI) (*appsv1.DaemonSet, error) {
	node := &appsv1.DaemonSet{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      constants.GetResourceName(constants.CSINode),
		Namespace: instance.Namespace,
	}, node)
	return node, err
}

func (r *IBMObjectCSIReconciler) getControllerDeployment(instance *crutils.IBMObjectCSI) (*appsv1.Deployment, error) {
	controllerDeployment := &appsv1.Deployment{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      constants.GetResourceName(constants.CSIController),
		Namespace: instance.Namespace,
	}, controllerDeployment)
	return controllerDeployment, err
}

func (r *IBMObjectCSIReconciler) reconcileClusterRoleBinding(instance *crutils.IBMObjectCSI) error {
	clusterRoleBindings := r.getClusterRoleBindings(instance)
	return r.ControllerHelper.ReconcileClusterRoleBinding(clusterRoleBindings)
}

func (r *IBMObjectCSIReconciler) reconcileStorageClasses(instance *crutils.IBMObjectCSI) error {
	logger := csiLog.WithValues("reconcileStorageClasses")
	logger.Info("Entry")
	defer logger.Info("Exit")
	storageClasses := r.getStorageClasses(instance)
	logger.Info("Number of storageclasses to be reonciled", len(storageClasses))
	return r.ControllerHelper.ReconcileStorageClasses(storageClasses)
}

func (r *IBMObjectCSIReconciler) reconcileClusterRole(instance *crutils.IBMObjectCSI) error {
	clusterRoles := r.getClusterRoles(instance)
	return r.ControllerHelper.ReconcileClusterRole(clusterRoles)
}

func (r *IBMObjectCSIReconciler) reconcileServiceAccount(instance *crutils.IBMObjectCSI) error {
	logger := csiLog.WithValues("Resource Type", "ServiceAccount")

	controller := instance.GenerateControllerServiceAccount()
	node := instance.GenerateNodeServiceAccount()

	controllerServiceAccountName := constants.GetResourceName(constants.CSIControllerServiceAccount)
	nodeServiceAccountName := constants.GetResourceName(constants.CSINodeServiceAccount)

	for _, sa := range []*corev1.ServiceAccount{
		controller,
		node,
	} {
		if err := controllerutil.SetControllerReference(instance.Unwrap(), sa, r.Scheme); err != nil {
			return err
		}
		found := &corev1.ServiceAccount{}
		err := r.Get(context.TODO(), types.NamespacedName{
			Name:      sa.Name,
			Namespace: sa.Namespace,
		}, found)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating a new ServiceAccount", "Namespace", sa.GetNamespace(), "Name", sa.GetName())
			err = r.Create(context.TODO(), sa)
			if err != nil {
				return err
			}

			nodeDaemonSet, err := r.getNodeDaemonSet(instance)
			if err != nil {
				return err
			}

			if controllerServiceAccountName == sa.Name {
				rErr := r.restartControllerPod(logger, instance)
				if rErr != nil {
					return rErr
				}
			}
			if nodeServiceAccountName == sa.Name {
				logger.Info("node rollout requires restart",
					"DesiredNumberScheduled", nodeDaemonSet.Status.DesiredNumberScheduled,
					"NumberAvailable", nodeDaemonSet.Status.NumberAvailable)
				logger.Info("csi node stopped being ready - restarting it")
				rErr := r.rolloutRestartNode(nodeDaemonSet)
				if rErr != nil {
					return rErr
				}
			}
		} else if err != nil {
			logger.Error(err, "Failed to get ServiceAccount", "Name", sa.GetName())
			return err
		} else {
			logger.Info("ServiceAccount already exists", "Namespace", sa.GetNamespace(), "Name", sa.GetName())
		}
	}

	return nil
}

func (r *IBMObjectCSIReconciler) rolloutRestartNode(node *appsv1.DaemonSet) error {
	restartedAt := fmt.Sprintf("%s/restartedAt", constants.APIGroup)
	timestamp := time.Now().String()
	node.Spec.Template.ObjectMeta.Annotations[restartedAt] = timestamp
	return r.Update(context.TODO(), node)
}

func (r *IBMObjectCSIReconciler) restartControllerPod(logger logr.Logger, instance *crutils.IBMObjectCSI) error {
	controllerDeployment, err := r.getControllerDeployment(instance)
	if err != nil {
		return err
	}

	logger.Info("controller requires restart",
		"ReadyReplicas", controllerDeployment.Status.ReadyReplicas,
		"Replicas", controllerDeployment.Status.Replicas)
	logger.Info("restarting csi controller")

	controllerPod, err := r.getControllerPod(controllerDeployment)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "failed to get controller pod")
		return err
	}

	return r.restartControllerPodfromDeployment(logger, controllerDeployment, controllerPod)
}

func (r *IBMObjectCSIReconciler) reconcileCSIDriver(instance *crutils.IBMObjectCSI) error {
	logger := csiLog.WithValues("Resource Type", "CSIDriver")

	cd := instance.GenerateCSIDriver()
	found := &storagev1.CSIDriver{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: cd.Name, Namespace: ""}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new CSIDriver", "Name", cd.GetName())
			err = r.Create(context.TODO(), cd)
			if err != nil {
				return err
			}
			logger.Info("CSIDriver created", "Namespace", cd.GetNamespace(), "Name", cd.GetName())
			return nil
		}
		logger.Error(err, "Failed to get CSIDriver", "Name", cd.GetName())
		return err
	}
	logger.Info("CSIDriver already exists", "Namespace", cd.GetNamespace(), "Name", cd.GetName())
	return nil
}

func (r *IBMObjectCSIReconciler) deleteCSIDriver(instance *crutils.IBMObjectCSI) error {
	logger := csiLog.WithName("deleteCSIDriver")

	csiDriver := instance.GenerateCSIDriver()
	found := &storagev1.CSIDriver{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      csiDriver.Name,
		Namespace: csiDriver.Namespace,
	}, found)
	if err == nil {
		logger.Info("deleting CSIDriver", "Name", csiDriver.GetName())
		if err := r.Delete(context.TODO(), found); err != nil {
			logger.Error(err, "failed to delete CSIDriver", "Name", csiDriver.GetName())
			return err
		}
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		logger.Error(err, "failed to get CSIDriver", "Name", csiDriver.GetName())
		return err
	}
	return nil
}

func (r *IBMObjectCSIReconciler) deleteClusterRoleBindings(instance *crutils.IBMObjectCSI) error {
	clusterRoleBindings := r.getClusterRoleBindings(instance)
	return r.ControllerHelper.DeleteClusterRoleBindings(clusterRoleBindings)
}

func (r *IBMObjectCSIReconciler) deleteStorageClasses(instance *crutils.IBMObjectCSI) error {
	storageClasses := r.getStorageClasses(instance)
	return r.ControllerHelper.DeleteStorageClasses(storageClasses)
}

func (r *IBMObjectCSIReconciler) getClusterRoleBindings(instance *crutils.IBMObjectCSI) []*rbacv1.ClusterRoleBinding {
	externalProvisioner := instance.GenerateExternalProvisionerClusterRoleBinding()
	controllerSCC := instance.GenerateSCCForControllerClusterRoleBinding()
	nodeSCC := instance.GenerateSCCForNodeClusterRoleBinding()

	return []*rbacv1.ClusterRoleBinding{
		externalProvisioner,
		controllerSCC,
		nodeSCC,
	}
}

func (r *IBMObjectCSIReconciler) getStorageClasses(instance *crutils.IBMObjectCSI) []*storagev1.StorageClass {
	var requiredRegion string

	s3Provider := r.ControllerHelper.GetS3Provider()

	k8sSCs := []*storagev1.StorageClass{}
	cosSCs := []string{}

	reclaimPolicys := []corev1.PersistentVolumeReclaimPolicy{
		corev1.PersistentVolumeReclaimRetain,
		corev1.PersistentVolumeReclaimDelete}

	if len(s3Provider) == 0 || s3Provider == constants.S3ProviderIBM {
		r.ControllerHelper.SetIBMCosEP()
		cosSCs = r.ControllerHelper.GetIBMCosSC()
		requiredRegion = r.ControllerHelper.GetRegion()
	} else {
		r.ControllerHelper.SetS3ProviderEP()
		cosSCs = append(cosSCs, "standard")
		requiredRegion = r.ControllerHelper.S3ProviderRegion
	}
	cosEP := r.ControllerHelper.GetCosEP()

	for _, sc := range cosSCs {
		for _, rp := range reclaimPolicys {
			k8sSc := instance.GenerateRcloneSC(rp, s3Provider, requiredRegion, cosEP, sc)
			k8sSCs = append(k8sSCs, k8sSc)

			k8sSc = instance.GenerateS3fsSC(rp, s3Provider, requiredRegion, cosEP, sc)
			k8sSCs = append(k8sSCs, k8sSc)
		}
	}
	return k8sSCs
}

func (r *IBMObjectCSIReconciler) deleteClusterRoles(instance *crutils.IBMObjectCSI) error {
	clusterRoles := r.getClusterRoles(instance)
	return r.ControllerHelper.DeleteClusterRoles(clusterRoles)
}

func (r *IBMObjectCSIReconciler) getClusterRoles(instance *crutils.IBMObjectCSI) []*rbacv1.ClusterRole {
	externalProvisioner := instance.GenerateExternalProvisionerClusterRole()
	controllerSCC := instance.GenerateSCCForControllerClusterRole()
	nodeSCC := instance.GenerateSCCForNodeClusterRole()

	return []*rbacv1.ClusterRole{
		externalProvisioner,
		controllerSCC,
		nodeSCC,
	}
}

func checkIfupdateCRFromConfigMapRequired(instance *objectdriverv1alpha1.IBMObjectCSI, cm *corev1.ConfigMap) bool {
	crUpdateRequired := false

	if val, ok := cm.Data[constants.NodeServerCPURequestCMKey]; ok {
		if instance.Spec.Node.Resources.Requests.CPU != val {
			instance.Spec.Node.Resources.Requests.CPU = val
			crUpdateRequired = true
		}
	}
	if val, ok := cm.Data[constants.NodeServerMemoryRequestCMKey]; ok {
		if instance.Spec.Node.Resources.Requests.Memory != val {
			instance.Spec.Node.Resources.Requests.Memory = val
			crUpdateRequired = true
		}
	}
	if val, ok := cm.Data[constants.NodeServerCPULimitCMKey]; ok {
		if instance.Spec.Node.Resources.Limits.CPU != val {
			instance.Spec.Node.Resources.Limits.CPU = val
			crUpdateRequired = true
		}
	}
	if val, ok := cm.Data[constants.NodeServerMemoryLimitCMKey]; ok {
		if instance.Spec.Node.Resources.Limits.Memory != val {
			instance.Spec.Node.Resources.Limits.Memory = val
			crUpdateRequired = true
		}
	}

	if val, ok := cm.Data[constants.MaxVolumesPerNodeCMKey]; ok {
		if instance.Spec.Node.MaxVolumesPerNode != val {
			instance.Spec.Node.MaxVolumesPerNode = val
			crUpdateRequired = true
		}
	}
	return crUpdateRequired
}

// SetupWithManager sets up the controller with the Manager.
func (r *IBMObjectCSIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectdriverv1alpha1.IBMObjectCSI{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&corev1.ConfigMap{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(configMapPredicate())).
		Complete(r)
}

func configMapPredicate() predicate.Predicate {
	logger := csiLog.WithName("configMapPredicate")
	triggerReconcile := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			configmap := e.Object.(*corev1.ConfigMap)
			if configmap.Namespace == constants.CSIOperatorNamespace && configmap.Name == constants.ParamsConfigMap {
				logger.Info("Configmap created", "configmap", configmap.Name)
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			configmap := e.ObjectNew.(*corev1.ConfigMap)
			if configmap.Namespace == constants.CSIOperatorNamespace && configmap.Name == constants.ParamsConfigMap {
				logger.Info("Update event on the configmap", "configmap", configmap.Name)
				return true
			}
			return false
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
	return triggerReconcile
}
