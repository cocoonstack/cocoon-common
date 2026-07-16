package k8s

import (
	"encoding/json"
	"testing"
)

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
		"vm.cocoonstack.io/hibernate": nil,
		"vm.cocoonstack.io/name":      "vk-demo-0",
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
	if got := patch.Metadata.Annotations["vm.cocoonstack.io/name"]; got != "vk-demo-0" {
		t.Fatalf("name mismatch: got %#v", got)
	}
	if got := patch.Metadata.Annotations["vm.cocoonstack.io/hibernate"]; got != nil {
		t.Fatalf("expected nil patch value, got %#v", got)
	}
}
