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
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/presslabs/controller-util/pkg/syncer"
	csiv1alpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	crutils "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/internal/crutils"
	clustersyncer "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/syncer"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/util/common"
	oconfig "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	oversion "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileTime is the delay between reconciliations
const ReconcileTime = 30 * time.Second

type reconciler func(instance *crutils.IBMObjectCSI) error

var csiLog = logf.Log.WithName("ibmobjectcsi_controller")

// IBMObjectCSIReconciler reconciles a IBMObjectCSI object
type IBMObjectCSIReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Namespace        string
	Recorder         record.EventRecorder
	ServerVersion    string
	ControllerHelper *common.ControllerHelper
}

//+kubebuilder:rbac:groups=csi.ibm.com,resources=ibmobjectcsis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ibm.com,resources=ibmobjectcsis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ibm.com,resources=ibmobjectcsis/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;delete;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;create;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;delete;list;watch;update;create;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;statefulsets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;delete;get;watch;list
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=create;delete;get;watch;list;update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resourceNames=ibm-object-csi-operator,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=create;delete;get;watch;list
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csinodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=security.openshift.io,resourceNames=anyuid;privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=create;list;watch;delete
// +kubebuilder:rbac:groups=csi.ibm.com,resources=*,verbs=*
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=create;get;list;watch;delete;update

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
	reqLogger.Info("Reconciling IBMObjectCSI")
	r.ControllerHelper.Log = csiLog

	// Fetch the CSIDriver instance
	instance := crutils.New(&csiv1alpha1.IBMObjectCSI{}, r.ServerVersion)
	err := r.Get(context.TODO(), req.NamespacedName, instance.Unwrap())
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
	changed := instance.SetDefaults()
	if err := instance.Validate(); err != nil {
		err = fmt.Errorf("wrong IBMObjectCSI options: %v", err)
		return reconcile.Result{RequeueAfter: ReconcileTime}, err
	}
	// update CR if there was changes after defaulting
	if changed {
		err = r.Update(context.TODO(), instance.Unwrap())
		if err != nil {
			err = fmt.Errorf("failed to update IBMObjectCSI CR: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	if err := r.ControllerHelper.AddFinalizerIfNotPresent(
		instance, instance.Unwrap()); err != nil {
		return reconcile.Result{}, err
	}
	// If the deletion timestamp is set, perform cleanup operations and remove a finalizer before returning from the reconciliation process.
	if !instance.GetDeletionTimestamp().IsZero() {
		isFinalizerExists, err := r.ControllerHelper.HasFinalizer(instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		// If the finalizer doesn't exist, return early, indicating that no further action is needed.

		if !isFinalizerExists {
			return reconcile.Result{}, nil
		}

		if err := r.deleteClusterRolesAndBindings(instance); err != nil {
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

	// create the resources which never change if not exist
	for _, rec := range []reconciler{
		r.reconcileCSIDriver,
		r.reconcileServiceAccount,
		r.reconcileClusterRole,
		r.reconcileClusterRoleBinding,
		r.reconcileStorageClasses,
	} {
		if err = rec(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// sync the resources which change over time
	csiControllerSyncer := clustersyncer.NewCSIControllerSyncer(r.Client, r.Scheme, instance)
	if err := syncer.Sync(context.TODO(), csiControllerSyncer, r.Recorder); err != nil {
		return reconcile.Result{}, err
	}

	csiNodeSyncer := clustersyncer.NewCSINodeSyncer(r.Client, r.Scheme, instance)
	if err := syncer.Sync(context.TODO(), csiNodeSyncer, r.Recorder); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatus(instance, originalStatus); err != nil {
		return reconcile.Result{}, err
	}

	// Resource created successfully - don't requeue

	return reconcile.Result{}, nil
}

func (r *IBMObjectCSIReconciler) updateStatus(instance *crutils.IBMObjectCSI, originalStatus csiv1alpha1.IBMObjectCSIStatus) error {
	logger := csiLog.WithName("updateStatus")
	controllerPod := &corev1.Pod{}
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
	phase := csiv1alpha1.DriverPhaseNone
	if instance.Status.ControllerReady && instance.Status.NodeReady {
		phase = csiv1alpha1.DriverPhaseRunning
	} else {
		if !instance.Status.ControllerReady {
			err := r.getControllerPod(controllerDeployment, controllerPod)
			if err != nil {
				logger.Error(err, "failed to get controller pod")
				return err
			}

			if !r.areAllPodImagesSynced(controllerDeployment, controllerPod) {
				r.restartControllerPodfromDeployment(logger, controllerDeployment, controllerPod)
			}
		}
		phase = csiv1alpha1.DriverPhaseCreating
	}
	instance.Status.Phase = phase
	instance.Status.Version = oversion.DriverVersion

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

func (r *IBMObjectCSIReconciler) getControllerPod(controllerDeployment *appsv1.Deployment, controllerPod *corev1.Pod) error {
	controllerPodName := fmt.Sprintf("%s-0", controllerDeployment.Name)
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      controllerPodName,
		Namespace: controllerDeployment.Namespace,
	}, controllerPod)
	if errors.IsNotFound(err) {
		return nil
	}
	return err
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
		Name:      oconfig.GetNameForResource(oconfig.CSINode, instance.Name),
		Namespace: instance.Namespace,
	}, node)

	return node, err
}

func (r *IBMObjectCSIReconciler) getControllerDeployment(instance *crutils.IBMObjectCSI) (*appsv1.Deployment, error) {
	controllerDeployment := &appsv1.Deployment{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      oconfig.GetNameForResource(oconfig.CSIController, instance.Name),
		Namespace: instance.Namespace,
	}, controllerDeployment)

	return controllerDeployment, err
}

func (r *IBMObjectCSIReconciler) reconcileClusterRoleBinding(instance *crutils.IBMObjectCSI) error {
	clusterRoleBindings := r.getClusterRoleBindings(instance)
	return r.ControllerHelper.ReconcileClusterRoleBinding(clusterRoleBindings)
}

func (r *IBMObjectCSIReconciler) reconcileStorageClasses(instance *crutils.IBMObjectCSI) error {
	storageClasses := r.getStorageClasses(instance)
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

	controllerServiceAccountName := oconfig.GetNameForResource(oconfig.CSIControllerServiceAccount, instance.Name)
	nodeServiceAccountName := oconfig.GetNameForResource(oconfig.CSINodeServiceAccount, instance.Name)

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
			// Resource already exists - don't requeue
			//logger.Info("Skip reconcile: ServiceAccount already exists", "Namespace", sa.GetNamespace(), "Name", sa.GetName())
		}
	}

	return nil
}

func (r *IBMObjectCSIReconciler) rolloutRestartNode(node *appsv1.DaemonSet) error {
	restartedAt := fmt.Sprintf("%s/restartedAt", oconfig.APIGroup)
	timestamp := time.Now().String()
	node.Spec.Template.ObjectMeta.Annotations[restartedAt] = timestamp
	return r.Update(context.TODO(), node)
}

func (r *IBMObjectCSIReconciler) restartControllerPod(logger logr.Logger, instance *crutils.IBMObjectCSI) error {
	controllerPod := &corev1.Pod{}
	controllerDeployment, err := r.getControllerDeployment(instance)
	if err != nil {
		return err
	}

	logger.Info("controller requires restart",
		"ReadyReplicas", controllerDeployment.Status.ReadyReplicas,
		"Replicas", controllerDeployment.Status.Replicas)
	logger.Info("restarting csi controller")

	err = r.getControllerPod(controllerDeployment, controllerPod)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		logger.Error(err, "failed to get controller pod")
		return err
	}

	return r.restartControllerPodfromDeployment(logger, controllerDeployment, controllerPod)
}

func (r *IBMObjectCSIReconciler) reconcileCSIDriver(instance *crutils.IBMObjectCSI) error {
	logger := csiLog.WithValues("Resource Type", "CSIDriver")

	cd := instance.GenerateCSIDriver()
	found := &storagev1.CSIDriver{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      cd.Name,
		Namespace: "",
	}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new CSIDriver", "Name", cd.GetName())
		err = r.Create(context.TODO(), cd)
		if err != nil {
			return err
		}
	} else if err != nil {
		logger.Error(err, "Failed to get CSIDriver", "Name", cd.GetName())
		return err
	} else {
		// Resource already exists - don't requeue
	}

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

func (r *IBMObjectCSIReconciler) deleteClusterRolesAndBindings(instance *crutils.IBMObjectCSI) error {
	if err := r.deleteClusterRoleBindings(instance); err != nil {
		return err
	}

	if err := r.deleteClusterRoles(instance); err != nil {
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
	rcloneSC := instance.GenerateRcloneSC()
	s3fsSC := instance.Generates3fsSC()
	return []*storagev1.StorageClass{
		rcloneSC,
		s3fsSC,
	}
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

// SetupWithManager sets up the controller with the Manager.
func (r *IBMObjectCSIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1alpha1.IBMObjectCSI{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ServiceAccount{}).
		Complete(r)
}
