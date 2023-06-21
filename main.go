package main

import (
	"context"
	"flag"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/go-logr/zapr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	controllerName = "my-controller"
)

type Operator struct {
	clientset       kubernetes.Interface
	apiextensionsv1 apiextensionsv1.Interface
	scheme          *runtime.Scheme
}

type pluginCRD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec pluginCRDSpec `json:"spec"`
}

type pluginCRDSpec struct {
	PVCName      string `json:"pvcName"`
	StorageClass string `json:"storageClass"`
	Bucket       string `json:"bucket"`
	Endpoint     string `json:"endpoint"`
}

func (crd *pluginCRD) DeepCopyObject() runtime.Object {
	return crd.DeepCopy()
}

func (crd *pluginCRD) DeepCopy() *pluginCRD {
	if crd == nil {
		return nil
	}
	copy := &pluginCRD{}
	copy.TypeMeta = crd.TypeMeta
	copy.ObjectMeta = crd.ObjectMeta
	copy.Spec = crd.Spec
	return copy
}

func NewOperator(clientset kubernetes.Interface, apiextensionsv1 apiextensionsv1.Interface, scheme *runtime.Scheme) *Operator {
	return &Operator{
		clientset:       clientset,
		apiextensionsv1: apiextensionsv1,
		scheme:          scheme,
	}
}

func (o *Operator) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Fetch the CRD object
	crd := &pluginCRD{}
	_, err := o.apiextensionsv1.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "my-crd", metav1.GetOptions{})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create or update the PVC
	err = o.reconcilePVC(ctx, crd)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create or update the Secret
	err = o.reconcileSecret(ctx, crd)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (o *Operator) reconcilePVC(ctx context.Context, crd *pluginCRD) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crd.Spec.PVCName,
			Namespace: crd.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &crd.Spec.StorageClass,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}

	// Set the CRD instance as the owner reference for the PVC
	if err := controllerutil.SetControllerReference(crd, pvc, o.scheme); err != nil {
		return err
	}

	// Check if the PVC exists
	_, err := o.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil {
		// PVC does not exist, create it
		_, err = o.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Created PersistentVolumeClaim '%s' in namespace '%s'", pvc.Name, pvc.Namespace)
	} else {
		// PVC already exists, update if necessary
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != crd.Spec.StorageClass {
			// Update the PVC storage class
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				result, err := o.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				result.Spec.StorageClassName = &crd.Spec.StorageClass
				_, updateErr := o.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(ctx, result, metav1.UpdateOptions{})
				return updateErr
			})
			if retryErr != nil {
				return retryErr
			}
			klog.Infof("Updated PersistentVolumeClaim '%s' in namespace '%s'", pvc.Name, pvc.Namespace)
		}

		// Check if the PV is bound and if the node is healthy
		if pvc.Status.Phase == corev1.ClaimBound {
			pvName := pvc.Spec.VolumeName
			pv, err := o.clientset.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get PersistentVolume '%s': %v", pvName, err)
			}

			// Check the health of the node on which PV is currently bound
			currentNodeName := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
			isNodeHealthy, err := o.isNodeHealthy(ctx, currentNodeName)
			if err != nil {
				return fmt.Errorf("failed to check node health: %v", err)
			}

			if !isNodeHealthy {
				// Find a healthy node and move the PV to it
				newNodeName, err := o.findHealthyNode(ctx)
				if err != nil {
					return fmt.Errorf("failed to find a healthy node: %v", err)
				}

				// Update the PV to move it to the new node
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0] = newNodeName
					_, updateErr := o.clientset.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
					return updateErr
				})
				if retryErr != nil {
					return retryErr
				}

				klog.Infof("Moved PersistentVolume '%s' from node '%s' to node '%s'", pvName, currentNodeName, newNodeName)
			}
		}
	}

	return nil
}

func (o *Operator) isNodeHealthy(ctx context.Context, nodeName string) (bool, error) {
	node, err := o.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get Node '%s': %v", nodeName, err)
	}

	// Check if the node is ready and in a healthy condition
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}

	return true, nil
}

func (o *Operator) findHealthyNode(ctx context.Context) (string, error) {
	nodes, err := o.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list Nodes: %v", err)
	}

	// Find a healthy node that is ready
	for _, node := range nodes.Items {
		isNodeHealthy, err := o.isNodeHealthy(ctx, node.Name)
		if err != nil {
			return "", fmt.Errorf("failed to check node health: %v", err)
		}
		if isNodeHealthy {
			return node.Name, nil
		}
	}

	return "", fmt.Errorf("no healthy node found")
}

func (o *Operator) reconcileSecret(ctx context.Context, crd *pluginCRD) error {
	secretName := crd.Spec.PVCName

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: crd.Namespace,
		},
		StringData: map[string]string{
			"bucket":   crd.Spec.Bucket,
			"endpoint": crd.Spec.Endpoint,
		},
	}

	// Set the CRD instance as the owner reference for the Secret
	if err := controllerutil.SetControllerReference(crd, secret, o.scheme); err != nil {
		return err
	}

	// Check if the Secret exists
	_, err := o.clientset.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		// Secret does not exist, create it
		_, err = o.clientset.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Created Secret '%s' in namespace '%s'", secret.Name, secret.Namespace)
	}

	return nil
}

func createResources(clientset *kubernetes.Clientset, namespace string) error {
	// Create ControllerServiceAccount if it doesn't exist
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), "controller-service-account", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ControllerServiceAccount: %v", err)
	} else if kerrors.IsNotFound(err) {
		controllerServiceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controller-service-account",
				Namespace: namespace,
			},
		}

		_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), controllerServiceAccount, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ControllerServiceAccount: %v", err)
		}
	}

	// Create DriverServiceAccount if it doesn't exist
	_, err = clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), "driver-service-account", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get DriverServiceAccount: %v", err)
	} else if kerrors.IsNotFound(err) {
		driverServiceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "driver-service-account",
				Namespace: namespace,
			},
		}

		_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), driverServiceAccount, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create DriverServiceAccount: %v", err)
		}
	}

	// Create ControllerClusterRole if it doesn't exist
	_, err = clientset.RbacV1().ClusterRoles().Get(context.TODO(), "controller-cluster-role", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ControllerClusterRole: %v", err)
	} else if kerrors.IsNotFound(err) {
		controllerClusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "controller-cluster-role",
			},
			Rules: []rbacv1.PolicyRule{
				// Define the desired rules for ControllerClusterRole
			},
		}

		_, err := clientset.RbacV1().ClusterRoles().Create(context.TODO(), controllerClusterRole, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ControllerClusterRole: %v", err)
		}
	}

	// Create DriverClusterRole if it doesn't exist
	_, err = clientset.RbacV1().ClusterRoles().Get(context.TODO(), "cluster-role-driver", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get DriverClusterRole: %v", err)
	} else if kerrors.IsNotFound(err) {
		driverClusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "driver-cluster-role",
			},
			Rules: []rbacv1.PolicyRule{
				// Define the desired rules for DriverClusterRole
			},
		}

		_, err := clientset.RbacV1().ClusterRoles().Create(context.TODO(), driverClusterRole, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create DriverClusterRole: %v", err)
		}
	}

	// Create ControllerClusterRoleBinding if it doesn't exist
	_, err = clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "cluster-role-binding-controller", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ControllerClusterRoleBinding: %v", err)
	} else if kerrors.IsNotFound(err) {
		controllerClusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-role-binding-controller",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "controller-service-account",
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "controller-cluster-role",
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		_, err := clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), controllerClusterRoleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ControllerClusterRoleBinding: %v", err)
		}
	}

	// Create DriverClusterRoleBinding if it doesn't exist
	_, err = clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "cluster-role-binding-driver", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get DriverClusterRoleBinding: %v", err)
	} else if kerrors.IsNotFound(err) {
		driverClusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-role-binding-driver",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "driver-service-account",
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "driver-cluster-role",
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		_, err := clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), driverClusterRoleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create DriverClusterRoleBinding: %v", err)
		}
	}

	// Create ControllerStatefulSet if it doesn't exist
	_, err = clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), "controller-statefulset", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ControllerStatefulSet: %v", err)
	} else if kerrors.IsNotFound(err) {
		controllerStatefulSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controller-statefulset",
				Namespace: namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ServiceAccountName: "controller-service-account",
						// Define the desired specification for ControllerStatefulSet
					},
				},
				// Define the desired specification for ControllerStatefulSet
			},
		}

		_, err := clientset.AppsV1().StatefulSets(namespace).Create(context.TODO(), controllerStatefulSet, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ControllerStatefulSet: %v", err)
		}
	}

	// Create DriverDaemonSet if it doesn't exist
	_, err = clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), "driver-daemonset", metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get DriverDaemonSet: %v", err)
	} else if kerrors.IsNotFound(err) {
		driverDaemonSet := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "driver-daemonset",
				Namespace: namespace,
			},
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ServiceAccountName: "driver-service-account",
						// Define the desired specification for DriverDaemonSet
					},
				},
				// Define the desired specification for DriverDaemonSet
			},
		}

		_, err := clientset.AppsV1().DaemonSets(namespace).Create(context.TODO(), driverDaemonSet, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create DriverDaemonSet: %v", err)
		}
	}

	return nil
}

func main() {
	kubeconfigPath := flag.String("kubeconfig", "", "path to the kubeconfig file")
	namespace := flag.String("namespace", "default", "namespace to watch for CRDs")
	flag.Parse()

	// Set up operator logging
	klogLogger := zap.NewRaw(zap.UseDevMode(false))
	logger := zapr.NewLogger(klogLogger)
	ctrl.SetLogger(logger)

	// Get the kubernetes cluster configuration
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		klog.Fatalf("Failed to get kubeconfig: %v", err)
	}

	// Create a new core clientset
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to create clientset: %v", err)
	}

	// Create the APIExtensions clientset
	apiExtensionsClientset, err := apiextensionsv1.NewForConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to create APIExtensions clientset: %v", err)
	}

	// Create a new manager
	mgr, err := manager.New(kubeconfig, manager.Options{})
	if err != nil {
		klog.Fatalf("Failed to create manager: %v", err)
	}

	// Create a new instance of crd operator
	operator := NewOperator(clientset, apiExtensionsClientset, mgr.GetScheme())

	// Initialize plugin driver and controller
	err = createResources(clientset, *namespace)
	if err != nil {
		panic(err.Error())
	}

	// Add crd operator to the manager
	err = ctrl.NewControllerManagedBy(mgr).
		For(&pluginCRD{}).
		Complete(operator)
	if err != nil {
		klog.Fatalf("Failed to add controller: %v", err)
	}

	// Start the manager
	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Fatalf("Failed to start manager: %v", err)
		}
	}()

	// Wait indefinitely
	select {}
}
