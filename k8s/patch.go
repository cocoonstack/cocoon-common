package k8s

import "encoding/json"

// MergePatch marshals a payload as a Kubernetes merge patch document.
func MergePatch(payload any) ([]byte, error) {
	return json.Marshal(payload)
}

// StatusMergePatch builds a merge patch for a status subresource update.
func StatusMergePatch(status any) ([]byte, error) {
	return MergePatch(map[string]any{"status": status})
}

// MetadataAnnotationsMergePatch builds a merge patch for annotation updates.
func MetadataAnnotationsMergePatch(annotations map[string]any) ([]byte, error) {
	return MergePatch(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})
}
