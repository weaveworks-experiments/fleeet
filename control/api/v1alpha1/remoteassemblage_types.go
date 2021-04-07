/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
)

// RemoteAssemblageSpec defines the desired state of RemoteAssemblage
type RemoteAssemblageSpec struct {
	// KubeconfigRef refers to a secret with a kubeconfig for the
	// remote cluster.
	// +required
	KubeconfigRef LocalKubeconfigReference `json:"kubeconfigRef"`

	// Assemblage gives the specification for the assemblage to create
	// downstream. It will be created with the same name as this
	// object.
	// +required
	Assemblage asmv1.AssemblageSpec `json:"assemblage"`
}

type LocalKubeconfigReference struct {
	// Name gives the name of the secret containing a kubeconfig.
	// +required
	Name string `json:"name"`
}

// RemoteAssemblageStatus defines the observed state of RemoteAssemblage
type RemoteAssemblageStatus struct {
	Syncs []asmv1.SyncStatus `json:"syncs,omitempty"`
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
