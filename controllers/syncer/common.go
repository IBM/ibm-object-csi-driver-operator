// Package syncer ...
package syncer

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/IBM/ibm-object-csi-driver-operator/controllers/constants"
)

var defaultAnnotations = []string{
	"productID",
	"productName",
	"productVersion",
}

// getEndpointEnvVars returns the IAM_ENDPOINT and COS_RESOURCE_CONFIG_ENDPOINT
// env vars appropriate for the given IaaS provider type (ibm-vpc, ibm-classic, or unknown).
func getEndpointEnvVars(iaaSProvider string) []corev1.EnvVar {
	var iamEP, cosRCEP string
	switch iaaSProvider {
	case constants.IaasIBMVPC:
		iamEP = constants.IAMEndpointVPC
		cosRCEP = constants.COSResourceConfigEndpointVPC
	case constants.IaasIBMClassic:
		iamEP = constants.IAMEndpointClassic
		cosRCEP = constants.COSResourceConfigEndpointClassic
	default:
		iamEP = constants.IAMEndpointUnknown
		cosRCEP = constants.COSResourceConfigEndpointUnknown
	}
	return []corev1.EnvVar{
		{Name: constants.EnvIAMEndpoint, Value: iamEP},
		{Name: constants.EnvCOSResourceConfigEndpoint, Value: cosRCEP},
	}
}

func ensureAnnotations(templateObjectMeta *metav1.ObjectMeta, objectMeta *metav1.ObjectMeta, annotations labels.Set) {
	for _, s := range defaultAnnotations {
		templateObjectMeta.Annotations[s] = annotations[s]
		objectMeta.Annotations[s] = annotations[s]
	}
}
