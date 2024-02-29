/*
Copyright 2024.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RestoreStaleVolumeSpec defines the desired state of RestoreStaleVolume

type RestoreStaleVolumeSpec struct {
	NoOfLogLines int64            `json:"noOfLogLines,omitempty"`
	Deployment   []DeploymentData `json:"deploymentData,omitempty"`
}

type DeploymentData struct {
	DeploymentName      string `json:"deploymentName,omitempty"`
	DeploymentNamespace string `json:"deploymentNamespace,omitempty"`
}

// RestoreStaleVolumeStatus defines the observed state of RestoreStaleVolume
type RestoreStaleVolumeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RestoreStaleVolume is the Schema for the recoverstalevolumes API
type RestoreStaleVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreStaleVolumeSpec   `json:"spec,omitempty"`
	Status RestoreStaleVolumeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RestoreStaleVolumeList contains a list of RestoreStaleVolume
type RestoreStaleVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestoreStaleVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RestoreStaleVolume{}, &RestoreStaleVolumeList{})
}
