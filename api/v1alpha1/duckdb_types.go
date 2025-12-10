/*
Copyright 2025.

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

// DuckDBSpec defines the desired state of DuckDB.
// +kubebuilder:subresource:status
type DuckDBSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Repository and tag of the DuckDB image to use.
	Image DuckDBImage `json:"image,omitempty"`

	Spec corev1.PodSpec `json:"spec,omitempty"`

	// Port defines the port to expose. Default is typically 8080 or similar for gizmosql.
	// +optional
	Port int32 `json:"port,omitempty"`
}

type DuckDBImage struct {
	Repository string            `json:"repository,omitempty"`
	Tag        string            `json:"tag,omitempty"`
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty" default:"IfNotPresent"`
}

// DuckDBStatus defines the observed state of DuckDB.
type DuckDBStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DuckDB is the Schema for the duckdbs API.
type DuckDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckDBSpec   `json:"spec,omitempty"`
	Status DuckDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DuckDBList contains a list of DuckDB.
type DuckDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DuckDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DuckDB{}, &DuckDBList{})
}
