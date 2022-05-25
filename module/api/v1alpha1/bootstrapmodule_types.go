/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	syncapi "github.com/squaremo/fleeet/pkg/api"
)

const KindBootstrapModule = "BootstrapModule"

// BootstrapModuleSpec defines the desired state of BootstrapModule
type BootstrapModuleSpec struct {
	// Selector gives the criteria for assigning this module to a
	// cluster. If missing, no clusters are selected. If present and
	// empty, all clusters are selected.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// ControlPlaneBindings gives bindings to evaluate in the control
	// plane, e.g., before applying to a worker cluster.
	ControlPlaneBindings []syncapi.Binding `json:"controlPlaneBindings,omitempty"`

	// Sync gives the configuration to sync on assigned clusters.
	// +required
	Sync syncapi.Sync `json:"sync"`
}

// BootstrapModuleStatus defines the observed state of BootstrapModule
type BootstrapModuleStatus struct {
	// ObservedSync gives the spec of the Sync as most recently acted
	// upon.
	// +optional
	ObservedSync *syncapi.Sync `json:"observedSync,omitempty"`
	// Summary gives the numbers of uses of the module that are in
	// various states at last count.
	// +optional
	Summary *SyncSummary `json:"summary,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.observedSync.source.git.version`
//+kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.summary.total`
//+kubebuilder:printcolumn:name="Updating",type=string,JSONPath=`.status.summary.updating`
//+kubebuilder:printcolumn:name="Succeeded",type=string,JSONPath=`.status.summary.succeeded`
//+kubebuilder:printcolumn:name="Failed",type=string,JSONPath=`.status.summary.failed`

// BootstrapModule is the Schema for the bootstrapmodules API
type BootstrapModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BootstrapModuleSpec   `json:"spec,omitempty"`
	Status BootstrapModuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BootstrapModuleList contains a list of BootstrapModule
type BootstrapModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BootstrapModule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BootstrapModule{}, &BootstrapModuleList{})
}
