// Package crutils ...
package crutils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Instance ...
type Instance interface {
	GetLabels() labels.Set
	GetObjectKind() schema.ObjectKind
}

// GetImagePullSecrets ...
func GetImagePullSecrets(imagePullSecrets []string) []corev1.LocalObjectReference {
	secrets := []corev1.LocalObjectReference{}
	if len(imagePullSecrets) > 0 {
		for _, secretName := range imagePullSecrets {
			secrets = append(secrets, corev1.LocalObjectReference{Name: secretName})
		}
	}
	return secrets
}

type SCInputParams struct {
	ReclaimPolicy   corev1.PersistentVolumeReclaimPolicy
	S3Provider      string
	Region          string
	COSEndpoint     string
	COSStorageClass string
	AddPerfSC       bool
}
