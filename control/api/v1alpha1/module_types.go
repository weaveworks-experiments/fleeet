/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
)

// ModuleSpec defines the desired state of Module
type ModuleSpec struct {
	// Selector gives the criteria for assigning this module to an
	// cluster. If missing, no clusters are selected. If present and
	// empty, all clusters are selected.
	// +optional
	Selector *metav1.LabelSelector `json:"selector"`

	// Sync gives the configuration to sync on assigned clusters.
	// +required
	Sync asmv1.Sync `json:"sync"`
}

// ModuleStatus defines the observed state of Module
type ModuleStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Module is the Schema for the modules API
type Module struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleSpec   `json:"spec,omitempty"`
	Status ModuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModuleList contains a list of Module
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Module `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Module{}, &ModuleList{})
}
