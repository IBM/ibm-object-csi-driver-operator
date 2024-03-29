package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/crutils"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/util"
	oconfig "github.com/IBM/ibm-object-csi-driver-operator/pkg/config"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type ControllerHelper struct {
	client.Client
	Log logr.Logger
}

func NewControllerHelper(client client.Client) *ControllerHelper {
	return &ControllerHelper{
		Client: client,
	}
}

func (ch *ControllerHelper) DeleteClusterRoleBindings(clusterRoleBindings []*rbacv1.ClusterRoleBinding) error {
	logger := ch.Log.WithName("DeleteClusterRoleBindings")
	for _, crb := range clusterRoleBindings {
		found, err := ch.getClusterRoleBinding(crb)
		if err != nil && errors.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get ClusterRoleBinding", "Name", crb.GetName())
			return err
		} else {
			logger.Info("deleting ClusterRoleBinding", "Name", crb.GetName())
			if err := ch.Delete(context.TODO(), found); err != nil {
				logger.Error(err, "failed to delete ClusterRoleBinding", "Name", crb.GetName())
				return err
			}
		}
	}
	return nil
}

func (ch *ControllerHelper) DeleteStorageClasses(storageClasses []*storagev1.StorageClass) error {
	logger := ch.Log.WithName("DeleteStorageClasses")
	for _, sc := range storageClasses {
		found, err := ch.getStorageClass(sc)
		if err != nil && errors.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get StorageClasses", "Name", sc.GetName())
			return err
		} else {
			logger.Info("deleting StorageClasses", "Name", sc.GetName())
			if err := ch.Delete(context.TODO(), found); err != nil {
				logger.Error(err, "failed to delete StorageClasses", "Name", sc.GetName())
				return err
			}
		}
	}
	return nil
}

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

func (ch *ControllerHelper) DeleteClusterRoles(clusterRoles []*rbacv1.ClusterRole) error {
	logger := ch.Log.WithName("DeleteClusterRoles")
	for _, cr := range clusterRoles {
		found, err := ch.getClusterRole(cr)
		if err != nil && errors.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get ClusterRole", "Name", cr.GetName())
			return err
		} else {
			logger.Info("deleting ClusterRole", "Name", cr.GetName())
			if err := ch.Delete(context.TODO(), found); err != nil {
				logger.Error(err, "failed to delete ClusterRole", "Name", cr.GetName())
				return err
			}
		}
	}
	return nil
}

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
	finalizerName := fmt.Sprintf("%s.%s", strings.ToLower(gvk.Kind), oconfig.APIGroup)

	accessor, err := meta.Accessor(instance)
	if err != nil {
		logger.Error(err, "failed to get meta information of instance")
		return nil, "", err
	}
	return accessor, finalizerName, nil
}
