// Package common ...
package common

import (
	"k8s.io/apimachinery/pkg/labels"
)

// GetSelectorLabels ...
func GetSelectorLabels(component string) labels.Set {
	return labels.Set{
		"app.kubernetes.io/component": component,
	}
}
