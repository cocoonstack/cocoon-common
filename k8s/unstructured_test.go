package k8s

import (
	"encoding/json"
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

func TestStatusMergePatch(t *testing.T) {
	data, err := StatusMergePatch(map[string]string{"phase": "Ready"})
	if err != nil {
		t.Fatalf("StatusMergePatch returned error: %v", err)
	}

	var patch map[string]map[string]string
	if err := json.Unmarshal(data, &patch); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	if got := patch["status"]["phase"]; got != "Ready" {
		t.Fatalf("status phase mismatch: got %q", got)
	}
}

func TestAnnotationsMergePatch(t *testing.T) {
	data, err := AnnotationsMergePatch(map[string]any{
		"cocoon.cis/hibernate": nil,
		"cocoon.cis/vm-name":   "vk-demo-0",
	})
	if err != nil {
		t.Fatalf("AnnotationsMergePatch returned error: %v", err)
	}

	var patch struct {
		Metadata struct {
			Annotations map[string]any `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &patch); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	if got := patch.Metadata.Annotations["cocoon.cis/vm-name"]; got != "vk-demo-0" {
		t.Fatalf("vm-name mismatch: got %#v", got)
	}
	if got := patch.Metadata.Annotations["cocoon.cis/hibernate"]; got != nil {
		t.Fatalf("expected nil patch value, got %#v", got)
	}
}
