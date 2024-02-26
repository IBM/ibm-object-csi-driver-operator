package crutils

import (
	"fmt"

	csiv1alpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/controllers/internal/common"
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	csiversion "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/version"
	"k8s.io/apimachinery/pkg/labels"
)

// IBMObjectCSI is the wrapper for csiv1alpha1.IBMObjectCSI type
type IBMObjectCSI struct {
	*csiv1alpha1.IBMObjectCSI
	ServerVersion string
}

// New returns a wrapper for csiv1.IBMObjectCSI
func New(c *csiv1alpha1.IBMObjectCSI, serverVersion string) *IBMObjectCSI {
	return &IBMObjectCSI{
		IBMObjectCSI:  c,
		ServerVersion: serverVersion,
	}
}

// Unwrap returns the csiv1.IBMObjectCSI object
func (c *IBMObjectCSI) Unwrap() *csiv1alpha1.IBMObjectCSI {
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

func (c *IBMObjectCSI) GetDefaultSidecarImageByName(name string) string {
	if sidecar, found := config.DefaultSidecarsByName[name]; found {
		return fmt.Sprintf("%s:%s", sidecar.Repository, sidecar.Tag)
	}
	return ""
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
