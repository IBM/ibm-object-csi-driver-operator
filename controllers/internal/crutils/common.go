package crutils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Instance interface {
	GetLabels() labels.Set
	GetObjectKind() schema.ObjectKind
}

func getImagePullSecrets(imagePullSecrets []string) []corev1.LocalObjectReference {
	secrets := []corev1.LocalObjectReference{}
	if len(imagePullSecrets) > 0 {
		for _, secretName := range imagePullSecrets {
			secrets = append(secrets, corev1.LocalObjectReference{Name: secretName})
		}
	}
	return secrets
}
