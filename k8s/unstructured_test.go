package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDecodeUnstructured(t *testing.T) {
	type spec struct {
		Name string `json:"name"`
	}
	type sample struct {
		Spec spec `json:"spec"`
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"name": "demo",
			},
		},
	}

	got, err := DecodeUnstructured[sample](obj)
	if err != nil {
		t.Fatalf("DecodeUnstructured returned error: %v", err)
	}
	if got.Spec.Name != "demo" {
		t.Fatalf("decoded name mismatch: got %q", got.Spec.Name)
	}
}
