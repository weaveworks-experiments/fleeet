package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-openapi/jsonpointer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrUnknownBindingForm = errors.New("unknown binding form")

// ResolveBinding finds a value given the specification of a
// binding. It expects a `client.Client` limited to the namespace of
// the owning object.
func ResolveBinding(ctx context.Context, client client.Client, b Binding) (string, error) {
	switch {
	case b.BindingSource.Value != nil:
		return *b.BindingSource.Value, nil
	case b.ObjectFieldRef != nil:
		obj, err := getArbitraryObject(ctx, client, b.ObjectFieldRef)
		if err != nil {
			return "", err
		}
		return evalFieldPath(obj.Object, b.ObjectFieldRef.FieldPath)
	default:
		return "", ErrUnknownBindingForm
	}
}

func getArbitraryObject(ctx context.Context, c client.Client, ref *ObjectFieldSelector) (unstructured.Unstructured, error) {
	obj := unstructured.Unstructured{}
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	err := c.Get(ctx, client.ObjectKey{
		Name: ref.Name,
	}, &obj)
	return obj, err
}

func evalFieldPath(obj map[string]interface{}, path string) (string, error) {
	ptr, err := jsonpointer.New(path)
	if err != nil {
		return "", err
	}
	val, kind, err := ptr.Get(obj)
	if err != nil {
		return "", err
	}
	switch kind {
	case reflect.String:
		return val.(string), nil
	default:
		// FIXME more cases, or at least be principled here ...
		return fmt.Sprintf("%v", val), nil
	}
}
