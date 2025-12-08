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

// DuckDBSpec defines the desired state of DuckDB
type DuckDBSpec struct {
	// Image allows overriding the default gizmodata/gizmosql image
	// +optional
	Image string `json:"image,omitempty"`

	// Replicas defines the number of desired pods.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Port defines the port to expose. Default is typically 8080 or similar for gizmosql.
	// +optional
	Port int32 `json:"port,omitempty"`
}

// DuckDBStatus defines the observed state of DuckDB
type DuckDBStatus struct {
	// AvailableReplicas indicates the number of available pods
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DuckDB is the Schema for the duckdbs API
type DuckDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckDBSpec   `json:"spec,omitempty"`
	Status DuckDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DuckDBList contains a list of DuckDB
type DuckDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DuckDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DuckDB{}, &DuckDBList{})
}
