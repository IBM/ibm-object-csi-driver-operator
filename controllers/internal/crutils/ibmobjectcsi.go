// Package crutils ...
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

// GetCSINodeSelectorLabels ...
func (c *IBMObjectCSI) GetCSINodeSelectorLabels() labels.Set {
	return common.GetSelectorLabels(config.CSINode.String())
}

// GetCSINodePodLabels ...
func (c *IBMObjectCSI) GetCSINodePodLabels() labels.Set {
	return labels.Merge(c.GetLabels(), c.GetCSINodeSelectorLabels())
}

// GetCSINodeImage ...
func (c *IBMObjectCSI) GetCSINodeImage() string {
	if c.Spec.Node.Tag == "" {
		return c.Spec.Node.Repository
	}
	return c.Spec.Node.Repository + ":" + c.Spec.Node.Tag
}

// GetCSIControllerSelectorLabels ...
func (c *IBMObjectCSI) GetCSIControllerSelectorLabels() labels.Set {
	return common.GetSelectorLabels(config.CSIController.String())
}

// GetCSIControllerPodLabels ...
func (c *IBMObjectCSI) GetCSIControllerPodLabels() labels.Set {
	return labels.Merge(c.GetLabels(), c.GetCSIControllerSelectorLabels())
}

// GetCSIControllerImage ...
func (c *IBMObjectCSI) GetCSIControllerImage() string {
	if c.Spec.Controller.Tag == "" {
		return c.Spec.Controller.Repository
	}
	return c.Spec.Controller.Repository + ":" + c.Spec.Controller.Tag
}

func (c *IBMObjectCSI) GetCSIControllerResourceRequests() *objectdriverv1alpha1.ResourcesSpec {
	resources := objectdriverv1alpha1.ResourcesSpec{}

	if &c.Spec.Controller.Resources != nil {
		resources = c.Spec.Controller.Resources
	}
	return &resources
}

func (c *IBMObjectCSI) GetCSINodeResourceRequests() *objectdriverv1alpha1.ResourcesSpec {
	resources := objectdriverv1alpha1.ResourcesSpec{}

	if &c.Spec.Node.Resources != nil {
		resources = c.Spec.Node.Resources
	}
	return &resources
}
