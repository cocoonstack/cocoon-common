package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// DecodeUnstructured converts an unstructured Kubernetes object into a typed value.
func DecodeUnstructured[T any](u *unstructured.Unstructured) (*T, error) {
	var out T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &out); err != nil {
		return nil, fmt.Errorf("decode %T: %w", out, err)
	}
	return &out, nil
}
