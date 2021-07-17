package api

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-openapi/jsonpointer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveBinding(client client.Client, b Binding) string {
	switch {
	case b.BindingSource.Value != nil:
		return *b.BindingSource.Value
	case b.ObjectFieldRef != nil:
		obj, err := getArbitraryObject(client, b.ObjectFieldRef)
		if err != nil {
			println("[DEBUG]", err.Error())
			return ""
		}
		return evalFieldPath(obj.Object, b.ObjectFieldRef.FieldPath)
	default:
		return ""
	}
}

func getArbitraryObject(c client.Client, ref *ObjectFieldSelector) (unstructured.Unstructured, error) {
	obj := unstructured.Unstructured{}
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	// FIXME Namespace? Perhaps pass a namespaced client.
	err := c.Get(context.TODO(), client.ObjectKey{
		Name: ref.Name,
	}, &obj)
	return obj, err
}

func evalFieldPath(obj map[string]interface{}, path string) string {
	ptr, err := jsonpointer.New(path)
	if err != nil {
		println("[DEBUG]", err.Error())
		return ""
	}
	val, kind, err := ptr.Get(obj)
	if err != nil {
		println("[DEBUG]", err.Error())
		return ""
	}
	switch kind {
	case reflect.String:
		return val.(string)
	default:
		// FIXME more cases, or at least be principled here ...
		return fmt.Sprintf("%v", val)
	}
}
