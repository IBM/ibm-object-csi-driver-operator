// Package syncer ...
package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
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

// getResourceRequirements converts a ResourcesSpec into corev1.ResourceRequirements.
// Requests and limits are only set when both CPU and memory are provided.
// Invalid quantities return an error instead of panicking.
func getResourceRequirements(res objectdriverv1alpha1.ResourcesSpec) (corev1.ResourceRequirements, error) {
	requests, err := parseResourceList(res.Requests)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid resource requests: %w", err)
	}
	limits, err := parseResourceList(res.Limits)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid resource limits: %w", err)
	}
	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}, nil
}

func parseResourceList(rl objectdriverv1alpha1.ReqLimits) (corev1.ResourceList, error) {
	if rl.CPU == "" || rl.Memory == "" {
		return nil, nil
	}
	cpu, err := resource.ParseQuantity(rl.CPU)
	if err != nil {
		return nil, fmt.Errorf("cpu %q: %w", rl.CPU, err)
	}
	memory, err := resource.ParseQuantity(rl.Memory)
	if err != nil {
		return nil, fmt.Errorf("memory %q: %w", rl.Memory, err)
	}
	return corev1.ResourceList{
		corev1.ResourceCPU:    cpu,
		corev1.ResourceMemory: memory,
	}, nil
}

func ensureAnnotations(templateObjectMeta *metav1.ObjectMeta, objectMeta *metav1.ObjectMeta, annotations labels.Set) {
	for _, s := range defaultAnnotations {
		templateObjectMeta.Annotations[s] = annotations[s]
		objectMeta.Annotations[s] = annotations[s]
	}
}
