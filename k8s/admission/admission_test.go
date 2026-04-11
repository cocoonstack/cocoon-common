package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAllow(t *testing.T) {
	if resp := Allow(); !resp.Allowed {
		t.Errorf("Allow should return Allowed=true")
	}
}

func TestDeny(t *testing.T) {
	resp := Deny("no")
	if resp.Allowed {
		t.Errorf("Deny should return Allowed=false")
	}
	if resp.Result == nil || resp.Result.Message != "no" {
		t.Errorf("Deny should carry the message: %+v", resp.Result)
	}
	if resp.Result.Reason != metav1.StatusReasonForbidden {
		t.Errorf("Deny reason: %q", resp.Result.Reason)
	}
}

func TestDecode(t *testing.T) {
	raw, err := json.Marshal(&admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{UID: "abc"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	review, err := Decode(r, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if review.Request == nil || review.Request.UID != "abc" {
		t.Errorf("decoded wrong: %+v", review)
	}
}

func TestServeCopiesUIDAndAllowsNilResponse(t *testing.T) {
	raw, err := json.Marshal(&admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{UID: "xyz"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	Serve(rr, r, 0, func(_ context.Context, _ *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
		return nil // should be normalized to Allow
	})

	body, _ := io.ReadAll(rr.Result().Body)
	_ = rr.Result().Body.Close()
	var out admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.Response == nil || !out.Response.Allowed {
		t.Errorf("nil handler result should be allowed, got %+v", out.Response)
	}
	if out.Response.UID != "xyz" {
		t.Errorf("response UID not copied: %q", out.Response.UID)
	}
}

func TestMarshalPatch(t *testing.T) {
	raw, err := MarshalPatch([]JSONPatchOp{
		{Op: "add", Path: "/metadata/annotations/x", Value: "y"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var ops []map[string]any
	if err := json.Unmarshal(raw, &ops); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ops) != 1 || ops[0]["op"] != "add" || ops[0]["value"] != "y" {
		t.Errorf("patch ops round trip: %v", ops)
	}
}

func TestEscapeJSONPointer(t *testing.T) {
	cases := map[string]string{
		"plain":                  "plain",
		"foo/bar":                "foo~1bar",
		"foo~bar":                "foo~0bar",
		"cocoonset.io/hibernate": "cocoonset.io~1hibernate",
	}
	for in, want := range cases {
		if got := EscapeJSONPointer(in); got != want {
			t.Errorf("EscapeJSONPointer(%q) = %q, want %q", in, got, want)
		}
	}
}
