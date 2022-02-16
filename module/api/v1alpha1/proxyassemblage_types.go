/*
Copyright 2021, 2022 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

// ProxyAssemblageSpec defines the desired state of ProxyAssemblage
type ProxyAssemblageSpec struct {
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

// RemoteAssemblageStatus defines the observed state of RemoteAssemblage
type ProxyAssemblageStatus struct {
	Syncs []syncapi.SyncStatus `json:"syncs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ProxyAssemblage is the Schema for the proxyassemblages API
type ProxyAssemblage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxyAssemblageSpec   `json:"spec,omitempty"`
	Status ProxyAssemblageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProxyAssemblageList contains a list of ProxyAssemblage
type ProxyAssemblageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxyAssemblage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxyAssemblage{}, &ProxyAssemblageList{})
}
