package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// DecodeUnstructured converts an Unstructured object into a typed struct.
func DecodeUnstructured[T any](u *unstructured.Unstructured) (*T, error) {
	out := new(T)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, out); err != nil {
		return nil, fmt.Errorf("decode %T: %w", out, err)
	}
	return out, nil
}
