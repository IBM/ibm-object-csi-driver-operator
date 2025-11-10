// Package common ...
package common

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	"github.com/go-logr/logr"
	openshiftclient "github.com/openshift/client-go/config/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ControllerHelper ...
type ControllerHelper struct {
	client.Client
	Log              logr.Logger
	Region           string
	CosEP            string // Regional COS Endpoint
	IaaSProvider     string
	S3Provider       string // IBM COS / AWS S3 / Wasabi
	S3ProviderRegion string
}

// NewControllerHelper ...
func NewControllerHelper(client client.Client, logger logr.Logger) *ControllerHelper {
	return &ControllerHelper{
		Client: client,
		Log:    logger,
	}
}

// DeleteClusterRoleBindings ...
func (ch *ControllerHelper) DeleteClusterRoleBindings(clusterRoleBindings []*rbacv1.ClusterRoleBinding) error {
	logger := ch.Log.WithName("DeleteClusterRoleBindings")
	for _, crb := range clusterRoleBindings {
		found, err := ch.getClusterRoleBinding(crb)
		if err != nil && k8sErr.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get ClusterRoleBinding", "Name", crb.GetName())
			return err
		}

		logger.Info("deleting ClusterRoleBinding", "Name", crb.GetName())
		if err := ch.Delete(context.TODO(), found); err != nil {
			logger.Error(err, "failed to delete ClusterRoleBinding", "Name", crb.GetName())
			return err
		}
	}
	return nil
}

// DeleteStorageClasses ...
func (ch *ControllerHelper) DeleteStorageClasses(storageClasses []*storagev1.StorageClass) error {
	logger := ch.Log.WithName("DeleteStorageClasses")
	for _, sc := range storageClasses {
		found, err := ch.getStorageClass(sc)
		if err != nil && k8sErr.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get StorageClasses", "Name", sc.GetName())
			return err
		}
		logger.Info("deleting StorageClasses", "Name", sc.GetName())
		if err := ch.Delete(context.TODO(), found); err != nil {
			logger.Error(err, "failed to delete StorageClasses", "Name", sc.GetName())
			return err
		}
	}
	return nil
}

// ReconcileClusterRoleBinding ...
func (ch *ControllerHelper) ReconcileClusterRoleBinding(clusterRoleBindings []*rbacv1.ClusterRoleBinding) error {
	logger := ch.Log.WithValues("Resource Type", "ClusterRoleBinding")
	for _, crb := range clusterRoleBindings {
		_, err := ch.getClusterRoleBinding(crb)
		if err != nil && k8sErr.IsNotFound(err) {
			logger.Info("Creating a new ClusterRoleBinding", "Name", crb.GetName())
			err = ch.Create(context.TODO(), crb)
			if err != nil {
				return err
			}
		} else if err != nil {
			logger.Error(err, "Failed to get ClusterRoleBinding", "Name", crb.GetName())
			return err
		}
		ch.Log.Info("Skip reconcile: ClusterRoleBinding already exists", "Name", crb.GetName())
	}
	return nil
}

// ReconcileStorageClasses ...
func (ch *ControllerHelper) ReconcileStorageClasses(storageclasses []*storagev1.StorageClass) error {
	logger := ch.Log.WithValues("Resource Type", "StorageClasses")
	for _, sc := range storageclasses {
		k8sSC, err := ch.getStorageClass(sc)
		if err != nil {
			if k8sErr.IsNotFound(err) {
				logger.Info("Creating a new StorageClass", "Name", sc.GetName())
				err = ch.Create(context.TODO(), sc)
				if err != nil {
					return err
				}
			} else {
				logger.Error(err, "Failed to get StorageClass", "Name", sc.GetName())
				return err
			}
		} else {
			patch := client.MergeFrom(k8sSC.DeepCopy())
			//Apply SC MountOptions related changes only to existing storageclasses, if applicable
			k8sSC.MountOptions = sc.MountOptions
			err = ch.Patch(context.TODO(), k8sSC, patch)
			if err != nil {
				logger.Error(err, "Failed to patch StorageClass", "Name", k8sSC.GetName())
				return err
			}
			logger.Info("Reconciled StorageClass", "Name", sc.GetName())
		}
	}
	return nil
}

func (ch *ControllerHelper) getClusterRoleBinding(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	found := &rbacv1.ClusterRoleBinding{}
	err := ch.Get(context.TODO(), types.NamespacedName{
		Name:      crb.Name,
		Namespace: crb.Namespace,
	}, found)
	return found, err
}

func (ch *ControllerHelper) getStorageClass(sc *storagev1.StorageClass) (*storagev1.StorageClass, error) {
	found := &storagev1.StorageClass{}
	err := ch.Get(context.TODO(), types.NamespacedName{
		Name:      sc.Name,
		Namespace: sc.Namespace,
	}, found)
	return found, err
}

// DeleteClusterRoles ...
func (ch *ControllerHelper) DeleteClusterRoles(clusterRoles []*rbacv1.ClusterRole) error {
	logger := ch.Log.WithName("DeleteClusterRoles")
	for _, cr := range clusterRoles {
		found, err := ch.getClusterRole(cr)
		if err != nil && k8sErr.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get ClusterRole", "Name", cr.GetName())
			return err
		}
		logger.Info("deleting ClusterRole", "Name", cr.GetName())
		if err := ch.Delete(context.TODO(), found); err != nil {
			logger.Error(err, "failed to delete ClusterRole", "Name", cr.GetName())
			return err
		}
	}
	return nil
}

// ReconcileClusterRole ...
func (ch *ControllerHelper) ReconcileClusterRole(clusterRoles []*rbacv1.ClusterRole) error {
	logger := ch.Log.WithValues("Resource Type", "ClusterRole")
	for _, cr := range clusterRoles {
		k8sCR, err := ch.getClusterRole(cr)
		if err != nil && k8sErr.IsNotFound(err) {
			logger.Info("Creating a new ClusterRole", "Name", cr.GetName())
			err = ch.Create(context.TODO(), cr)
			if err != nil {
				return err
			}
		} else if err != nil {
			logger.Error(err, "Failed to get ClusterRole", "Name", cr.GetName())
			return err
		} else {
			patch := client.MergeFrom(k8sCR.DeepCopy())
			k8sCR.Rules = cr.Rules
			err = ch.Patch(context.TODO(), k8sCR, patch)
			if err != nil {
				logger.Error(err, "Failed to patch ClusterRole", "Name", k8sCR.GetName())
				return err
			}
		}
	}
	return nil
}

func (ch *ControllerHelper) getClusterRole(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	found := &rbacv1.ClusterRole{}
	err := ch.Get(context.TODO(), types.NamespacedName{
		Name:      cr.GetName(),
		Namespace: cr.GetNamespace(),
	}, found)
	return found, err
}

// AddFinalizerIfNotPresent ...
func (ch *ControllerHelper) AddFinalizerIfNotPresent(instance crutils.Instance,
	unwrappedInstance client.Object) error {
	logger := ch.Log.WithName("AddFinalizerIfNotPresent")

	accessor, finalizerName, err := ch.getAccessorAndFinalizerName(instance, unwrappedInstance)
	if err != nil {
		return err
	}

	err = ch.updateControllerFinalizer(context.TODO(), constants.AddFinalizer, finalizerName)
	if err != nil {
		return err
	}

	if !util.Contains(accessor.GetFinalizers(), finalizerName) {
		logger.Info("adding finalizer to CR", "CR name", accessor.GetName(), "finalizer", finalizerName)
		accessor.SetFinalizers(append(accessor.GetFinalizers(), finalizerName))

		if err := ch.Update(context.TODO(), unwrappedInstance); err != nil {
			logger.Error(err, "failed to add finalizer to CR", "CR name", accessor.GetName(), "finalizer", finalizerName)
			return err
		}
		logger.Info("finalizer added to CR", "CR name", accessor.GetName(), "finalizer", finalizerName)
	} else {
		logger.Info("finalizer already present on CR. Ignoring...", "CR name", accessor.GetName(), "finalizer", finalizerName)
	}
	return nil
}

// RemoveFinalizer ...
func (ch *ControllerHelper) RemoveFinalizer(instance crutils.Instance,
	unwrappedInstance client.Object) error {
	logger := ch.Log.WithName("RemoveFinalizer")

	accessor, finalizerName, err := ch.getAccessorAndFinalizerName(instance, unwrappedInstance)
	if err != nil {
		return err
	}

	if !util.Contains(accessor.GetFinalizers(), finalizerName) {
		logger.Info("finalizer already removed from CR. Ignoring...", "CR name", accessor.GetName(), "finalizer", finalizerName)
	} else {
		logger.Info("removing finalizer from CR", "CR name", accessor.GetName(), "finalizer", finalizerName)
		accessor.SetFinalizers(util.Remove(accessor.GetFinalizers(), finalizerName))

		// Remove old finalizer for backward compatibility, if present
		oldFinalizer := "ibmobjectcsi.objectdriver.csi.ibm.com"
		accessor.SetFinalizers(util.Remove(accessor.GetFinalizers(), oldFinalizer))

		if err := ch.Update(context.TODO(), unwrappedInstance); err != nil {
			logger.Error(err, "failed to remove finalizer from CR", "CR name", accessor.GetName(), "finalizer", finalizerName)
			return err
		}
		logger.Info("finalizer removed from CR", "CR name", accessor.GetName(), "finalizer", finalizerName)
	}

	err = ch.updateControllerFinalizer(context.TODO(), constants.RemoveFinalizer, finalizerName)
	if err != nil {
		return err
	}
	return nil
}

func (ch *ControllerHelper) getAccessorAndFinalizerName(instance crutils.Instance, unwrappedInstance client.Object) (metav1.Object, string, error) {
	logger := ch.Log.WithName("getAccessorAndFinalizerName")

	gvk, err := apiutil.GVKForObject(unwrappedInstance, ch.Scheme())
	if err != nil {
		logger.Error(err, "failed to get group version kink information of instance")
		return nil, "", err
	}
	finalizerName := fmt.Sprintf("%s.%s/finalizer", strings.ToLower(gvk.Kind), constants.APIGroup)

	accessor, err := meta.Accessor(instance)
	if err != nil {
		logger.Error(err, "failed to get meta information of instance")
		return nil, "", err
	}
	return accessor, finalizerName, nil
}

// Check the platform, if IBMCloud then get Region and IaaS provider
// If not IBMCloud, check if it is unmanaged/IPI cluster
func (ch *ControllerHelper) GetClusterInfo(inConfig rest.Config) error {
	logger := ch.Log.WithName("getClusterInfo")
	logger.Info("Checking cluster platform...")
	var listOptions = &client.ListOptions{}
	var err error
	nodes := corev1.NodeList{}

	k8sClient, err := kubernetes.NewForConfig(&inConfig)
	if err != nil {
		logger.Error(err, "Unable to load cluster config")
		return err
	}

	if k8sClient != nil {
		var list *corev1.NodeList
		list, err = k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			nodes = *list
		}
	} else {
		err = ch.List(context.TODO(), &nodes, listOptions)
	}
	if err != nil {
		logger.Error(err, "Get Cluster Info")
		return err
	}

	if len(nodes.Items) == 0 {
		err := errors.New("cluster nodes not found")
		logger.Error(err, "failed to fetch cluster nodes list")
		return err
	}

	logger.Info("Get IBM cluster region...")
	if val, ok := nodes.Items[0].Labels["ibm-cloud.kubernetes.io/region"]; ok {
		ch.Region = val
		logger.Info("Detected", "IBM Cluster region: ", ch.Region)
	} else {
		logger.Info("Node label 'ibm-cloud.kubernetes.io/region' not found")
	}

	logger.Info("Get IBM cluster IaaS Provider...")
	if val, ok := nodes.Items[0].Labels["ibm-cloud.kubernetes.io/iaas-provider"]; ok {
		logger.Info("Detected", "IBM IaaS provider: ", val)
		if val == "g2" {
			ch.IaaSProvider = constants.IaasIBMVPC
		} else {
			ch.IaaSProvider = constants.IaasIBMClassic
		}
		logger.Info("Detected", "endpoint type: ", ch.IaaSProvider)
	} else {
		logger.Info("Node label 'ibm-cloud.kubernetes.io/iaas-provider' not found")
	}

	if ch.Region == "" || ch.IaaSProvider == "" {
		// check if it is unmanaged cluster
		logger.Info("Region or IaaSProvider not set. checking if it is unmanaged cluster")

		ocClient, err := openshiftclient.NewForConfig(&inConfig)
		if err != nil {
			logger.Error(err, "Unable to load cluster config")
			return err
		}

		infra, err := ocClient.ConfigV1().Infrastructures().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "Failed to get infrastructure")
			return err
		}

		logger.Info("cluster infrastructure", "platformStatus:", infra.Status.PlatformStatus)

		platformType := infra.Status.PlatformStatus.Type
		logger.Info("Detected", "infra cloud provider platform: ", platformType)

		if platformType == constants.InfraProviderPlatformIBM {
			logger.Info("Get cluster region...")
			region := infra.Status.PlatformStatus.IBMCloud.Location
			if region != "" {
				ch.Region = region
				logger.Info("Detected", "Cluster region: ", ch.Region)
			} else {
				logger.Info("cluster region not found")
			}

			logger.Info("Get cluster Provider...")
			providerType := infra.Status.PlatformStatus.IBMCloud.ProviderType
			if providerType != "" {
				logger.Info("Detected", "IaaS provider: ", providerType)
				if providerType == constants.InfraProviderType {
					ch.IaaSProvider = constants.IaasIBMVPC
				} else {
					ch.IaaSProvider = constants.IaasIBMClassic
				}
				logger.Info("Detected", "endpoint type: ", ch.IaaSProvider)
			} else {
				logger.Info("cluster IaaS provider not found")
			}
		} else {
			logger.Info("cloud provider is not IBMCloud")
		}
	}
	return nil
}

func (ch *ControllerHelper) GetS3Provider() string {
	return ch.S3Provider
}

func (ch *ControllerHelper) GetIaaSProvider() string {
	return ch.IaaSProvider
}

func (ch *ControllerHelper) GetRegion() string {
	return ch.Region
}

func (ch *ControllerHelper) GetCosEP() string {
	return ch.CosEP
}

func (ch *ControllerHelper) GetIBMCosSC() []string {
	if len(ch.IaaSProvider) == 0 || len(ch.Region) == 0 {
		return make([]string, 0)
	}
	cosSC := []string{"standard", "smart"}
	return cosSC
}

func (ch *ControllerHelper) SetIBMCosEP() {
	if len(ch.IaaSProvider) == 0 || len(ch.Region) == 0 {
		ch.CosEP = ""
	}
	if ch.IaaSProvider == constants.IaasIBMVPC || ch.IaaSProvider == constants.IaasIBMClassic {
		epType := "private"
		if ch.IaaSProvider == constants.IaasIBMVPC {
			epType = "direct"
		}
		ch.CosEP = fmt.Sprintf(constants.IBMEP, epType, ch.Region)
	}
}

func (ch *ControllerHelper) SetS3ProviderEP() {
	if ch.S3ProviderRegion == "" {
		ch.CosEP = ""
	}

	if ch.S3Provider == constants.S3ProviderAWS {
		ch.CosEP = fmt.Sprintf(constants.AWSEP, ch.S3ProviderRegion)
	}

	if ch.S3Provider == constants.S3ProviderWasabi {
		ch.CosEP = fmt.Sprintf(constants.WasabiEP, ch.S3ProviderRegion)
	}
}

// Update finalizer to Controller Deployment under NS "ibm-object-csi-operator"
// op = 1  Add finalizer    op = 2  Remove finalizer
func (ch *ControllerHelper) updateControllerFinalizer(ctx context.Context, op constants.FinalizerOps, finalizerName string) error {
	logger := ch.Log.WithValues("name", constants.DeploymentName, "namespace", constants.CSIOperatorNamespace, "finalizer", finalizerName)
	logger.Info("updateControllerFinalizer: Entry")
	defer logger.Info("updateControllerFinalizer: Exit")

	ctrlDep := &appsv1.Deployment{}
	err := ch.Get(ctx, client.ObjectKey{Namespace: constants.CSIOperatorNamespace, Name: constants.DeploymentName}, ctrlDep)
	if err != nil {
		logger.Error(err, "updateControllerFinalizer: controller deployment not found. Retrying...")
		return err
	}

	if op == constants.AddFinalizer { // Add finalizer
		logger.Info("updateControllerFinalizer: adding finalizer to controller deployment")
		if exists := controllerutil.ContainsFinalizer(ctrlDep, finalizerName); !exists {
			controllerutil.AddFinalizer(ctrlDep, finalizerName)
			err = ch.Update(ctx, ctrlDep)
			if err != nil {
				logger.Error(err, "updateControllerFinalizer: failed to add finalizer to controller deployment")
				return err
			}
			logger.Info("updateControllerFinalizer: finalizer has been added to controller deployment")
		} else {
			logger.Info("updateControllerFinalizer: finalizer already present in controller deployment. Ignoring...")
		}
	}

	if op == constants.RemoveFinalizer { // Remove finalizer
		logger.Info("updateControllerFinalizer: removing finalizer from controller deployment")
		if exists := controllerutil.ContainsFinalizer(ctrlDep, finalizerName); exists {
			controllerutil.RemoveFinalizer(ctrlDep, finalizerName)
			err = ch.Update(ctx, ctrlDep)
			if err != nil {
				logger.Error(err, "updateControllerFinalizer: failed to remove finalizer from controller deployment")
				return err
			}
			logger.Info("updateControllerFinalizer: finalizer has been removed from controller deployment")
		} else {
			logger.Info("updateControllerFinalizer: finalizer not found in controller deployment. Ignoring...")
		}
	}

	return nil
}
