package k8s

import "encoding/json"

// StatusMergePatch builds a merge patch for a status subresource update.
func StatusMergePatch(status any) ([]byte, error) {
	return json.Marshal(map[string]any{"status": status})
}

// AnnotationsMergePatch builds a merge patch for annotation updates.
func AnnotationsMergePatch(annotations map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})
}
