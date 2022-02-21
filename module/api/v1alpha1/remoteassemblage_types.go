/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	syncapi "github.com/squaremo/fleeet/pkg/api"
)

// RemoteAssemblageSpec defines the desired state of RemoteAssemblage
type RemoteAssemblageSpec struct {
	// KubeconfigRef refers to a secret with a kubeconfig for the
	// remote cluster.
	// +required
	KubeconfigRef LocalKubeconfigReference `json:"kubeconfigRef"`

	// Syncs gives the list of sync specs, each specifying a config to apply to the remote cluster.
	// +optional
	Syncs []RemoteSync `json:"syncs,omitempty"`
}

// SourceReference is a reference to supply to the Flux sync primitive created. Sources are shared
// amongst assemblages, rather than created per assemblage.
type SourceReference struct {
	// Name gives the name of the source (which is assumed to be in the same namespace as the
	// referrer).
	// +required
	Name string `json:"name"`
	// APIVersion gives the API group and version of the source object, e.g.,
	// `source.toolkit.fluxcd.io/v1beta2`
	// +required
	APIVersion string `json:"apiVersion"`
	// Kind gives the kind of the source object, e.g., `GitRepository`
	// +required
	Kind string `json:"kind"`
}

type RemoteSync struct {
	// Name gives a name to use for this sync, so that updates can be stable (changing the sync spec
	// will update objects rather than replace them)
	// +required
	Name string `json:"name"`
	// ControlPlaneBindings gives a list of variable bindings to evaluate when constructing the sync primitives
	// +optional
	ControlPlaneBindings []syncapi.Binding `json:"controlPlaneBindings,omitempty"`
	// SourceRef gives a reference to the source to use in the sync primitive
	// +required
	SourceRef SourceReference `json:"sourceRef"`
	// Package defines how the sources is to be applied; e.g., by kustomize
	// +required
	// +kubebuilder:default={"kustomize": {"path": "."}}
	Package *syncapi.PackageSpec `json:"package,omitempty"`
}

type LocalKubeconfigReference struct {
	// Name gives the name of the secret containing a kubeconfig.
	// +required
	Name string `json:"name"`
}

// RemoteAssemblageStatus defines the observed state of RemoteAssemblage
type RemoteAssemblageStatus struct {
	Syncs []syncapi.SyncStatus `json:"syncs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RemoteAssemblage is the Schema for the remoteassemblages API
type RemoteAssemblage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteAssemblageSpec   `json:"spec,omitempty"`
	Status RemoteAssemblageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RemoteAssemblageList contains a list of RemoteAssemblage
type RemoteAssemblageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteAssemblage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteAssemblage{}, &RemoteAssemblageList{})
}
