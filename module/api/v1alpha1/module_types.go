/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	syncapi "github.com/squaremo/fleeet/pkg/api"
)

const KindModule = "Module"

// ModuleSpec defines the desired state of Module
type ModuleSpec struct {
	// Selector gives the criteria for assigning this module to a
	// cluster. If missing, no clusters are selected. If present and
	// empty, all clusters are selected.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// ControlPlaneBindings gives bindings to evaluate in the control
	// plane, i.e., before the sync is "sent" to each worker
	// cluster.
	ControlPlaneBindings []syncapi.Binding `json:"controlPlaneBindings,omitempty"`

	// Sync gives the configuration to sync on assigned clusters.
	// +required
	Sync SyncWithBindings `json:"sync"`
}

// SyncWithBindings is a pairing of a sync (source and package) with
// bindings that will be evaluated in the target cluster.
type SyncWithBindings struct {
	Bindings     []syncapi.Binding `json:"bindings,omitempty"`
	syncapi.Sync `json:",inline"`
}

// ModuleStatus defines the observed state of Module
type ModuleStatus struct {
	// ObservedSync gives the spec of the Sync as most recently acted
	// upon.
	// +optional
	ObservedSync *syncapi.Sync `json:"observedSync,omitempty"`
	// Summary gives the numbers of uses of the module that are in
	// various states at last count.
	// +optional
	Summary *SyncSummary `json:"summary,omitempty"`
}

type SyncSummary struct {
	// Total gives the total number of assemblages using this module.
	Total int `json:"total"`
	// Updating gives the number of uses of this module that are in
	// progress updating to the most recent module spec, and not yet
	// synced.
	Updating int `json:"updating"`
	// Failed gives the number of uses of this module that are in a
	// failed state.
	Failed int `json:"failed"`
	// Succeeded gives the number of uses of this module that are in a
	// succeeded state.
	Succeeded int `json:"succeeded"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.observedSync.source.git.version`
//+kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.summary.total`
//+kubebuilder:printcolumn:name="Updating",type=string,JSONPath=`.status.summary.updating`
//+kubebuilder:printcolumn:name="Succeeded",type=string,JSONPath=`.status.summary.succeeded`
//+kubebuilder:printcolumn:name="Failed",type=string,JSONPath=`.status.summary.failed`

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
