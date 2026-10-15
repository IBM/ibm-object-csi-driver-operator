package syncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	objectdriverv1alpha1 "github.com/IBM/ibm-object-csi-driver-operator/api/v1alpha1"
)

func TestGetResourceRequirements(t *testing.T) {
	testCases := []struct {
		name        string
		input       objectdriverv1alpha1.ResourcesSpec
		expected    corev1.ResourceRequirements
		expectedErr string
	}{
		{
			name: "valid requests and limits",
			input: objectdriverv1alpha1.ResourcesSpec{
				Requests: objectdriverv1alpha1.ReqLimits{CPU: "40m", Memory: "40Mi"},
				Limits:   objectdriverv1alpha1.ReqLimits{CPU: "200m", Memory: "200Mi"},
			},
			expected: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("40m"),
					corev1.ResourceMemory: resource.MustParse("40Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
		},
		{
			name:     "empty spec",
			input:    objectdriverv1alpha1.ResourcesSpec{},
			expected: corev1.ResourceRequirements{},
		},
		{
			name: "only cpu set is skipped",
			input: objectdriverv1alpha1.ResourcesSpec{
				Requests: objectdriverv1alpha1.ReqLimits{CPU: "40m"},
			},
			expected: corev1.ResourceRequirements{},
		},
		{
			name: "invalid request cpu",
			input: objectdriverv1alpha1.ResourcesSpec{
				Requests: objectdriverv1alpha1.ReqLimits{CPU: "abc", Memory: "40Mi"},
			},
			expectedErr: `invalid resource requests: cpu "abc"`,
		},
		{
			name: "invalid limit memory",
			input: objectdriverv1alpha1.ResourcesSpec{
				Limits: objectdriverv1alpha1.ReqLimits{CPU: "200m", Memory: "lots"},
			},
			expectedErr: `invalid resource limits: memory "lots"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getResourceRequirements(tc.input)
			if tc.expectedErr != "" {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErr)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}
