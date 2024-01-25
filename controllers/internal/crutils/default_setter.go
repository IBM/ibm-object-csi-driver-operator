package crutils

import (
	"github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

// SetDefaults set defaults if omitted in spec, returns true means CR should be updated on cluster.
// Replace it with kubernetes native default setter when it is available.
// https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#defaulting
func (c *IBMObjectCSI) SetDefaults() bool {

	c.setDefaultForNilSliceFields()

	return c.setDefaults()
}

func (c *IBMObjectCSI) setDefaults() bool {
	var changed = false

	if c.Spec.Controller.Repository != config.DefaultIBMObjectCSICr.Spec.Controller.Repository ||
		c.Spec.Controller.Tag != config.DefaultIBMObjectCSICr.Spec.Controller.Tag {
		c.Spec.Controller.Repository = config.DefaultIBMObjectCSICr.Spec.Controller.Repository
		c.Spec.Controller.Tag = config.DefaultIBMObjectCSICr.Spec.Controller.Tag

		changed = true
	}

	if c.Spec.Node.Repository != config.DefaultIBMObjectCSICr.Spec.Node.Repository ||
		c.Spec.Node.Tag != config.DefaultIBMObjectCSICr.Spec.Node.Tag {
		c.Spec.Node.Repository = config.DefaultIBMObjectCSICr.Spec.Node.Repository
		c.Spec.Node.Tag = config.DefaultIBMObjectCSICr.Spec.Node.Tag

		changed = true
	}

	changed = c.setDefaultSidecars() || changed

	return changed
}

func (c *IBMObjectCSI) setDefaultForNilSliceFields() {
	if c.Spec.ImagePullSecrets == nil {
		c.Spec.ImagePullSecrets = []string{}
	}
	if c.Spec.Controller.Tolerations == nil {
		c.Spec.Controller.Tolerations = []corev1.Toleration{}
	}
	if c.Spec.Node.Tolerations == nil {
		c.Spec.Node.Tolerations = []corev1.Toleration{}
	}
}

func (c *IBMObjectCSI) setDefaultSidecars() bool {
	var change = false
	var defaultSidecars = config.DefaultIBMObjectCSICr.Spec.Sidecars

	if len(defaultSidecars) == len(c.Spec.Sidecars) {
		for _, sidecar := range c.Spec.Sidecars {
			if defaultSidecar, found := config.DefaultSidecarsByName[sidecar.Name]; found {
				if sidecar != defaultSidecar {
					change = true
				}
			} else {
				change = true
			}
		}
	} else {
		change = true
	}

	if change {
		c.Spec.Sidecars = defaultSidecars
	}

	return change
}
