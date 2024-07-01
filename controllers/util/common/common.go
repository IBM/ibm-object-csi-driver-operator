// Package common ...
package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// ControllerHelper ...
type ControllerHelper struct {
	client.Client
	Log              logr.Logger
	Region           string
	CosEP            string
	IaaSProvider     string
	S3Provider       string // IBM COS / AWS S3 / Wasabi
	S3ProviderRegion string
}

// NewControllerHelper ...
func NewControllerHelper(client client.Client) *ControllerHelper {
	return &ControllerHelper{
		Client: client,
	}
}

// DeleteClusterRoleBindings ...
func (ch *ControllerHelper) DeleteClusterRoleBindings(clusterRoleBindings []*rbacv1.ClusterRoleBinding) error {
	logger := ch.Log.WithName("DeleteClusterRoleBindings")
	for _, crb := range clusterRoleBindings {
		found, err := ch.getClusterRoleBinding(crb)
		if err != nil && errors.IsNotFound(err) {
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
		if err != nil && errors.IsNotFound(err) {
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
		if err != nil && errors.IsNotFound(err) {
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
		_, err := ch.getStorageClass(sc)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating a new StorageClass", "Name", sc.GetName())
			err = ch.Create(context.TODO(), sc)
			if err != nil {
				return err
			}
		} else if err != nil {
			logger.Error(err, "Failed to get StorageClass", "Name", sc.GetName())
			return err
		}
		ch.Log.Info("Skip reconcile: StorageClass already exists", "Name", sc.GetName())
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
		if err != nil && errors.IsNotFound(err) {
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
		_, err := ch.getClusterRole(cr)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating a new ClusterRole", "Name", cr.GetName())
			err = ch.Create(context.TODO(), cr)
			if err != nil {
				return err
			}
		} else if err != nil {
			logger.Error(err, "Failed to get ClusterRole", "Name", cr.GetName())
			return err
		} else {
			err = ch.Update(context.TODO(), cr)
			if err != nil {
				logger.Error(err, "Failed to update ClusterRole", "Name", cr.GetName())
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

	if !util.Contains(accessor.GetFinalizers(), finalizerName) {
		logger.Info("adding", "finalizer", finalizerName, "on", accessor.GetName())
		accessor.SetFinalizers(append(accessor.GetFinalizers(), finalizerName))

		if err := ch.Update(context.TODO(), unwrappedInstance); err != nil {
			logger.Error(err, "failed to add", "finalizer", finalizerName, "on", accessor.GetName())
			return err
		}
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

	accessor.SetFinalizers(util.Remove(accessor.GetFinalizers(), finalizerName))
	if err := ch.Update(context.TODO(), unwrappedInstance); err != nil {
		logger.Error(err, "failed to remove", "finalizer", finalizerName, "from", accessor.GetName())
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
	finalizerName := fmt.Sprintf("%s.%s", strings.ToLower(gvk.Kind), constants.APIGroup)

	accessor, err := meta.Accessor(instance)
	if err != nil {
		logger.Error(err, "failed to get meta information of instance")
		return nil, "", err
	}
	return accessor, finalizerName, nil
}

// Check the platform, if IBM Cloud then get Region and IaaS provider
func (ch *ControllerHelper) GetIBMClusterInfo(clientset *kubernetes.Clientset) error {
	var listOptions = &client.ListOptions{}
	var err error
	nodes := corev1.NodeList{}

	logger := ch.Log.WithName("getClusterInfo")
	logger.Info("Checking cluster platform...")

	if clientset != nil {
		list, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
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

	logger.Info("Get cluster region...")
	if val, ok := nodes.Items[0].Labels["ibm-cloud.kubernetes.io/region"]; ok {
		ch.Region = val
		logger.Info("Detected IBM Cluster region: ", ch.Region)
	}

	logger.Info("Get cluster IaaS Provider...")
	if val, ok := nodes.Items[0].Labels["ibm-cloud.kubernetes.io/iaas-provider"]; ok {
		logger.Info("Detected IBM IaaS provider: ", val)
		if val == "g2" {
			ch.IaaSProvider = constants.IaasIBMVPC
		} else {
			ch.IaaSProvider = constants.IaasIBMClassic
		}
		logger.Info("Detected endpoint type: ", ch.IaaSProvider)
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

func (ch *ControllerHelper) IsIBMColud() bool {
	retVal := false
	if len(ch.IaaSProvider) == 0 || len(ch.Region) == 0 {
		return retVal
	}
	if ch.IaaSProvider == constants.IaasIBMVPC || ch.IaaSProvider == constants.IaasIBMClassic {
		retVal = true
	}
	return retVal
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
		ch.S3ProviderRegion = ""
	}
	if ch.IaaSProvider == constants.IaasIBMVPC || ch.IaaSProvider == constants.IaasIBMClassic {
		epType := "private"
		if ch.IaaSProvider == constants.IaasIBMVPC {
			epType = "direct"
		}
		ch.S3ProviderRegion = fmt.Sprintf("https://s3.%s.%s.cloud-object-storage.appdomain.cloud", epType, ch.Region)
	}
}

func (ch *ControllerHelper) SetS3ProviderEP() {
	if ch.S3ProviderRegion == "" {
		ch.S3ProviderRegion = ""
	}

	if ch.S3Provider == constants.S3ProviderAWS {
		ch.S3ProviderRegion = fmt.Sprintf("https://s3.%s.amazonaws.com", ch.S3ProviderRegion)
	}

	if ch.S3Provider == constants.S3ProviderWasabi {
		ch.S3ProviderRegion = fmt.Sprintf("https://s3.%s.wasabisys.com", ch.S3ProviderRegion)
	}
}
