package common

import "k8s.io/apimachinery/pkg/labels"

func GetSelectorLabels(component string) labels.Set {
	return labels.Set{
		"app.kubernetes.io/component": component,
	}
}
