package crutils

import (
	"fmt"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/common"
	"github.com/IBM/ibm-object-csi-driver-operator/pkg/config"
	csiversion "github.com/IBM/ibm-object-csi-driver-operator/version"
	"k8s.io/apimachinery/pkg/labels"
)

// IBMObjectCSI is the wrapper for objectdriverv1alpha1.IBMObjectCSI type
type IBMObjectCSI struct {
	*objectdriverv1alpha1.IBMObjectCSI
}

// New returns a wrapper for objectdriverv1.IBMObjectCSI
func New(c *objectdriverv1alpha1.IBMObjectCSI) *IBMObjectCSI {
	return &IBMObjectCSI{
		IBMObjectCSI: c,
	}
}

// Unwrap returns the objectdriverv1.IBMObjectCSI object
func (c *IBMObjectCSI) Unwrap() *objectdriverv1alpha1.IBMObjectCSI {
	return c.IBMObjectCSI
}

// GetLabels returns all the labels to be set on all resources
func (c *IBMObjectCSI) GetLabels() labels.Set {
	labels := labels.Set{
		"app.kubernetes.io/name":       config.ProductName,
		"app.kubernetes.io/instance":   c.Name,
		"app.kubernetes.io/version":    csiversion.Version,
		"app.kubernetes.io/managed-by": config.Name,
		"product":                      config.ProductName,
		"release":                      fmt.Sprintf("v%s", csiversion.Version),
	}

	if c.Labels != nil {
		for k, v := range c.Labels {
			if !labels.Has(k) {
				labels[k] = v
			}
		}
	}

	return labels
}

// GetAnnotations returns all the annotations to be set on all resources
func (c *IBMObjectCSI) GetAnnotations() labels.Set {
	labels := labels.Set{
		"productID":      config.ProductName,
		"productName":    config.ProductName,
		"productVersion": csiversion.Version,
	}

	if c.Annotations != nil {
		for k, v := range c.Annotations {
			if !labels.Has(k) {
				labels[k] = v
			}
		}
	}

	return labels
}

func (c *IBMObjectCSI) GetCSINodeSelectorLabels() labels.Set {
	return common.GetSelectorLabels(config.CSINode.String())
}

func (c *IBMObjectCSI) GetCSINodePodLabels() labels.Set {
	return labels.Merge(c.GetLabels(), c.GetCSINodeSelectorLabels())
}

func (c *IBMObjectCSI) GetCSINodeImage() string {
	if c.Spec.Node.Tag == "" {
		return c.Spec.Node.Repository
	}
	return c.Spec.Node.Repository + ":" + c.Spec.Node.Tag
}

func (c *IBMObjectCSI) GetCSIControllerSelectorLabels() labels.Set {
	return common.GetSelectorLabels(config.CSIController.String())
}

func (c *IBMObjectCSI) GetCSIControllerPodLabels() labels.Set {
	return labels.Merge(c.GetLabels(), c.GetCSIControllerSelectorLabels())
}

func (c *IBMObjectCSI) GetCSIControllerImage() string {
	if c.Spec.Controller.Tag == "" {
		return c.Spec.Controller.Repository
	}
	return c.Spec.Controller.Repository + ":" + c.Spec.Controller.Tag
}

func (c *IBMObjectCSI) GetCSIControllerResourceRequests() *objectdriverv1alpha1.ResourcesSpec {
	//limits, requests := make(map[string]string), make(map[string]string)
	resources := objectdriverv1alpha1.ResourcesSpec{}

	if &c.Spec.Controller.Resources != nil {

		resources = c.Spec.Controller.Resources

		//limits["cpu"] = c.Spec.Controller.Resources.Limits.Cpu
		//limits["memory"] = c.Spec.Controller.Resources.Limits.Memory
		//requests["cpu"] = c.Spec.Controller.Resources.Requests.Cpu
		//requests["memory"] = c.Spec.Controller.Resources.Requests.Memory
		//
		//resources.Limits = objectdriverv1alpha1.ReqLimits{
		//	Cpu:    limits["cpu"],
		//	Memory: limits["memory"],
		//}
		//resources.Requests = objectdriverv1alpha1.ReqLimits{
		//	Cpu:    requests["cpu"],
		//	Memory: requests["memory"],
		//}
	}

	return &resources
}

func (c *IBMObjectCSI) GetCSINodeResourceRequests() *objectdriverv1alpha1.ResourcesSpec {
	//limits, requests := make(map[string]string), make(map[string]string)
	resources := objectdriverv1alpha1.ResourcesSpec{}

	//if &c.Spec.Node.Resources != nil {
	//	limits["cpu"] = c.Spec.Node.Resources.Limits.Cpu
	//	limits["memory"] = c.Spec.Node.Resources.Limits.Memory
	//	requests["cpu"] = c.Spec.Node.Resources.Requests.Cpu
	//	requests["memory"] = c.Spec.Node.Resources.Requests.Memory
	//
	//	resources.Limits = objectdriverv1alpha1.ReqLimits{
	//		Cpu:    limits["cpu"],
	//		Memory: limits["memory"],
	//	}
	//	resources.Requests = objectdriverv1alpha1.ReqLimits{
	//		Cpu:    requests["cpu"],
	//		Memory: requests["memory"],
	//	}
	//}
	if &c.Spec.Node.Resources != nil {
		resources = c.Spec.Node.Resources
	}

	return &resources
}

func (c *IBMObjectCSI) GetCSIResource(image string) string {
	if c.Spec.Controller.Tag == "" {
		return c.Spec.Controller.Repository
	}
	return c.Spec.Controller.Repository + ":" + c.Spec.Controller.Tag
}
