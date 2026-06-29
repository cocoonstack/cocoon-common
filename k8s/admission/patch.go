package admission

import (
	"encoding/json"
	"fmt"
	"strings"
)

// jsonPointerReplacer escapes ~ and / per RFC 6901.
var jsonPointerReplacer = strings.NewReplacer("~", "~0", "/", "~1")

// JSONPatchOp represents a single RFC 6902 JSON Patch operation.
type JSONPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// MarshalPatch encodes patch ops as JSON for AdmissionResponse.Patch.
func MarshalPatch(ops []JSONPatchOp) ([]byte, error) {
	out, err := json.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("marshal patch: %w", err)
	}
	return out, nil
}

// EscapeJSONPointer escapes ~ and / per RFC 6901 for use in patch paths.
func EscapeJSONPointer(s string) string {
	return jsonPointerReplacer.Replace(s)
}
