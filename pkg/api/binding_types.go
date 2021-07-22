/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package api

// Binding specifies how to obtain a value to bind to a name. The name
// can then be mentioned elsewhere in an object, and be replaced with
// the value as evaluated.
type Binding struct {
	// +required
	Name string `json:"name"`
	// +required
	BindingSource `json:",inline"`
}

// BindingSource is a union of the various places a value can come from
type BindingSource struct {
	// Value supplies a literal value
	// +optional
	*StringValue `json:",omitempty"`
	// +optional
	ObjectFieldRef *ObjectFieldSelector `json:"objectFieldRef,omitempty"`
}

type StringValue struct {
	// +optional
	Value string `json:"value"`
}

type ObjectFieldSelector struct {
	// APIVersion gives the APIVersion (<group>/<version>) for the object's type
	// +optional
	APIVersion string `json:"apiVersion"`
	// Kind gives the kind of the object's type
	// +required
	Kind string `json:"kind"`
	// Name names the object
	// +required
	Name string `json:"name"`
	// Path is a JSONPointer expression for finding the value in the
	// object identified
	// +required
	FieldPath string `json:"fieldPath"`
}
