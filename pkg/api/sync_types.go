/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package api

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

// NamedSync is used when there's a list of syncs, so the name can be
// mentioned elsewhere to refer to the particular sync. These always
// have their own bindings because they are used in types that have
// bindings to evaluate in the target cluster.
type NamedSync struct {
	// Name gives the sync a name so it can be correlated to the status
	// +required
	Name string `json:"name"`
	// Bindings gives a list of variable bindings to use when evaluating the package spec in the sync
	// +optional
	Bindings []Binding `json:"bindings,omitempty"`
	// +required
	Sync `json:",inline"`
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
	// Substitute gives a map of names to values to substitute in the
	// YAML built from the kustomization.
	// +optional
	Substitute map[string]string `json:"substitute,omitempty"`
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
