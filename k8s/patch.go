package k8s

import "encoding/json"

// AnnotationsMergePatch builds a merge patch for annotation updates.
func AnnotationsMergePatch(annotations map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	})
}
