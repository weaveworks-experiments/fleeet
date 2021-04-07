/*
Copyright 2021 Michael Bridgen
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssemblageSpec defines the desired state of Assemblage
type AssemblageSpec struct {
	// +required
	Syncs []NamedSync `json:"syncs"`
}

type NamedSync struct {
	// Name gives the sync a name so it can be correlated to the status
	// +required
	Name string `json:"name"`
	Sync `json:",inline"`
}

// Sync defines a versioned piece of configuration to be synced, and
// how to sync it.
type Sync struct {
	// Source gives the specification for how to get the configuration
	// to be synced
	// +required
	Source SourceSpec `json:"source"`

	// Package defines how to deal with the configuration at the
	// source, e.g., if it's a kustomization (or YAML files)
	// +optional
	// +kubebuilder:default={"kustomize": {"path": "."}}
	Package *PackageSpec `json:"package,omitempty"`
}

// SourceSpec gives the details for the source, i.e., from where to
// get the configuration
type SourceSpec struct {
	// +required
	Git *GitSource `json:"git"`
}

type GitSource struct {
	// URL gives the URL for the git repository
	// +required
	URL string `json:"url"`

	// Version gives either the revision or tag at which to get the
	// git repo
	// +required
	Version GitVersion `json:"version"`
}

type GitVersion struct {
	// +optional
	Tag string `json:"tag,omitempty"`
	// +optional
	Revision string `json:"revision,omitempty"`
}

// PackageSpec is a union of different kinds of configuration
type PackageSpec struct {
	// +optional
	Kustomize *KustomizeSpec `json:"kustomize,omitempty"`
}

type KustomizeSpec struct {
	// Path gives the path within the source to treat as the
	// Kustomization root.
	// +optional
	// +kubebuilder:default=.
	Path string `json:"path,omitempty"`
}

// AssemblageStatus defines the observed state of Assemblage
type AssemblageStatus struct {
	Syncs []SyncStatus `json:"syncs,omitempty"`
}

type SyncState string

const (
	// Synced successfully
	StateSucceeded SyncState = "succeeded"
	// Synced unsuccessfully
	StateFailed SyncState = "failed"
	// Updating in progress
	StateUpdating SyncState = "updating"
)

// SyncStatus gives the status of a specific sync.
type SyncStatus struct {
	// Sync gives the last applied sync spec.
	Sync NamedSync `json:"sync"`
	// State gives the outcome of last applied sync spec.
	State SyncState `json:"state"`
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
