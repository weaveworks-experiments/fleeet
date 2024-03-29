package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-openapi/jsonpointer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/squaremo/fleeet/pkg/expansion"
)

var ErrUnknownBindingForm = errors.New("unknown binding form")

// ResolveBinding finds a value given the specification of a
// binding. It expects a `client.Client` limited to the namespace of
// the owning object.
func ResolveBinding(ctx context.Context, client client.Client, b Binding, resolve func(string) string) (string, error) {
	switch {
	case b.BindingSource.StringValue != nil:
		s := b.BindingSource.StringValue.Value
		s = expansion.Expand(s, resolve)
		return s, nil
	case b.ObjectFieldRef != nil:
		ref := *b.ObjectFieldRef
		ref.APIVersion = expansion.Expand(ref.APIVersion, resolve)
		ref.Kind = expansion.Expand(ref.Kind, resolve)
		ref.Name = expansion.Expand(ref.Name, resolve)
		ref.FieldPath = expansion.Expand(ref.FieldPath, resolve)
		obj, err := getArbitraryObject(ctx, client, &ref)
		if err != nil {
			return "", err
		}
		return evalFieldPath(obj.Object, ref.FieldPath)
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
