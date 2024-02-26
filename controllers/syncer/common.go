package syncer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var defaultAnnotations = []string{
	"productID",
	"productName",
	"productVersion",
}

func ensureAnnotations(templateObjectMeta *metav1.ObjectMeta, objectMeta *metav1.ObjectMeta, annotations labels.Set) {
	for _, s := range defaultAnnotations {
		templateObjectMeta.Annotations[s] = annotations[s]
		objectMeta.Annotations[s] = annotations[s]
	}
}
