package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCocoonSetSpecSnapshotCompatibilityClassJSON(t *testing.T) {
	spec := CocoonSetSpec{SnapshotCompatibilityClass: "n2-cascade-lake-v1"}
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal CocoonSetSpec: %v", err)
	}
	if !strings.Contains(string(raw), `"snapshotCompatibilityClass":"n2-cascade-lake-v1"`) {
		t.Fatalf("snapshot compatibility class missing from JSON: %s", raw)
	}

	raw, err = json.Marshal(CocoonSetSpec{})
	if err != nil {
		t.Fatalf("marshal empty CocoonSetSpec: %v", err)
	}
	if strings.Contains(string(raw), "snapshotCompatibilityClass") {
		t.Fatalf("empty snapshot compatibility class must be omitted: %s", raw)
	}
}
