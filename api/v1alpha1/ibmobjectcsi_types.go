/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// IBMObjectCSISpec defines the desired state of IBMObjectCSI
type IBMObjectCSISpec struct {
	Controller IBMObjectCSIControllerSpec `json:"controller"`
	Node       IBMObjectCSINodeSpec       `json:"node"`

	// +kubebuilder:validation:Optional
	Sidecars []CSISidecar `json:"sidecars,omitempty"`

	// +kubebuilder:validation:Optional
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	HealthPort uint16 `json:"healthPort,omitempty"`
}

// IBMObjectCSINodeSpec defines the desired state of IBMObjectCSINode
type IBMObjectCSINodeSpec struct {
	// ObjectCSIComponent `json:"objectCSIComponent"`

	Repository string `json:"repository"`
	Tag        string `json:"tag"`

	// +kubebuilder:validation:Optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// IBMObjectCSIControllerSpec defines the desired state of IBMObjectCSIController
type IBMObjectCSIControllerSpec struct {
	// ObjectCSIComponent `json:"objectCSIComponent"`

	Repository string `json:"repository"`
	Tag        string `json:"tag"`

	// +kubebuilder:validation:Optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// IBMObjectCSIStatus defines the observed state of IBMObjectCSI
type IBMObjectCSIStatus struct {
	// Phase is the driver running phase
	Phase           DriverPhase `json:"phase"`
	ControllerReady bool        `json:"controllerReady"`
	NodeReady       bool        `json:"nodeReady"`

	// Version is the current driver version
	Version string `json:"version"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// IBMObjectCSI is the Schema for the ibmobjectcsis API
type IBMObjectCSI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMObjectCSISpec   `json:"spec,omitempty"`
	Status IBMObjectCSIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IBMObjectCSIList contains a list of IBMObjectCSI
type IBMObjectCSIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMObjectCSI `json:"items"`
}

type CSISidecar struct {
	// The name of the csi sidecar image
	Name string `json:"name"`

	// The repository of the csi sidecar image
	Repository string `json:"repository"`

	// The tag of the csi sidecar image
	Tag string `json:"tag"`

	// The pullPolicy of the csi sidecar image
	// +kubebuilder:validation:Optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`
}

func init() {
	SchemeBuilder.Register(&IBMObjectCSI{}, &IBMObjectCSIList{})
}
