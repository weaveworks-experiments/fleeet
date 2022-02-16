/*
Copyright 2021, 2022 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProxyAssemblageSpec defines the desired state of ProxyAssemblage
type ProxyAssemblageSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ProxyAssemblage. Edit proxyassemblage_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// ProxyAssemblageStatus defines the observed state of ProxyAssemblage
type ProxyAssemblageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
