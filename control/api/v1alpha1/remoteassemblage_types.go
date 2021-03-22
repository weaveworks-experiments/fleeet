/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RemoteAssemblageSpec defines the desired state of RemoteAssemblage
type RemoteAssemblageSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of RemoteAssemblage. Edit remoteassemblage_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// RemoteAssemblageStatus defines the observed state of RemoteAssemblage
type RemoteAssemblageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
