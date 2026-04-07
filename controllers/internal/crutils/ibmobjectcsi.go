// Package crutils ...
package crutils

import (
	"fmt"
	"strings"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
	"github.com/IBM/ibm-object-csi-driver-operator/controllers/internal/common"
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

// GetVersion returns the release version from CR annotation
func (c *IBMObjectCSI) GetVersion() string {
	if c.Annotations != nil {
		if release, ok := c.Annotations["release"]; ok && release != "" {
			return strings.TrimPrefix(release, "v")
		}
	}
	return "unknown"
}

// GetLabels returns all the labels to be set on all resources
func (c *IBMObjectCSI) GetLabels() labels.Set {
	version := c.GetVersion()

	labels := labels.Set{
		"app.kubernetes.io/name":       constants.CSIDriverName,
		"app.kubernetes.io/instance":   c.Name,
		"app.kubernetes.io/version":    version,
		"app.kubernetes.io/part-of":    constants.CSIDriverName,
		"app.kubernetes.io/managed-by": constants.CSIOperatorName,
		"product":                      constants.CSIDriverName,
		"release":                      fmt.Sprintf("v%s", version),
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
	version := c.GetVersion()

	labels := labels.Set{
		"productID":      constants.CSIDriverName,
		"productName":    constants.CSIDriverName,
		"productVersion": version,
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
	return common.GetSelectorLabels(constants.CSINode)
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
	return common.GetSelectorLabels(constants.CSIController)
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

// GetCSIControllerResourceRequests ...
func (c *IBMObjectCSI) GetCSIControllerResourceRequests() *objectdriverv1alpha1.ResourcesSpec {
	return &c.Spec.Controller.Resources
}

// GetCSINodeResourceRequests ...
func (c *IBMObjectCSI) GetCSINodeResourceRequests() *objectdriverv1alpha1.ResourcesSpec {
	return &c.Spec.Node.Resources
}
