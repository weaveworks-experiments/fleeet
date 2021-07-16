/*
Copyright 2021 Michael Bridgen
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	syncapi "github.com/squaremo/fleeet/pkg/api"
)

// AssemblageSpec defines the desired state of Assemblage
type AssemblageSpec struct {
	// +optional
	Bindings []syncapi.Binding `json:"bindings,omitempty"`
	// +required
	Syncs []syncapi.NamedSync `json:"syncs"`
}

// AssemblageStatus defines the observed state of Assemblage
type AssemblageStatus struct {
	Syncs []syncapi.SyncStatus `json:"syncs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Assemblage is the Schema for the assemblages API
type Assemblage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssemblageSpec   `json:"spec,omitempty"`
	Status AssemblageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AssemblageList contains a list of Assemblage
type AssemblageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Assemblage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Assemblage{}, &AssemblageList{})
}
