// Package admission hosts the shared admission-webhook scaffolding
// used by cocoon-webhook and any future cocoonstack webhook. It
// covers the AdmissionReview decode / dispatch / encode loop,
// allow / deny response builders, and a minimal RFC 6902 JSON patch
// helper.
//
// Logger: the Serve path uses projecteru2/core/log (the same logger
// every cocoon binary already initializes via cocoon-common/log),
// so handlers do not need to pass one in.
package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/projecteru2/core/log"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultMaxBody is the request-body ceiling Serve applies when the
// caller passes 0. 10 MiB is the same upper bound cocoon-webhook
// used before the helper moved to cocoon-common.
const DefaultMaxBody int64 = 10 << 20

// Handler is the admission function shape Serve dispatches to. Each
// handler receives the decoded review and returns a response; a nil
// response is normalized to Allow() on the write side so handlers
// can "return nil" as a success shortcut.
type Handler func(ctx context.Context, review *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// Allow returns a permissive AdmissionResponse with no patch.
func Allow() *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{Allowed: true}
}

// Deny returns a forbidden AdmissionResponse carrying msg as the
// human-readable status message.
func Deny(msg string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
			Reason:  metav1.StatusReasonForbidden,
		},
	}
}

// Decode parses an AdmissionReview from an HTTP request body,
// rejecting payloads larger than maxBytes. A zero or negative
// maxBytes falls back to DefaultMaxBody so callers that don't care
// about the ceiling can pass 0.
func Decode(r *http.Request, maxBytes int64) (*admissionv1.AdmissionReview, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBody
	}
	var review admissionv1.AdmissionReview
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBytes)).Decode(&review); err != nil {
		return nil, err
	}
	return &review, nil
}

// Serve decodes an AdmissionReview, dispatches it to handler, copies
// the request UID onto the response (required by the API server),
// and writes the response. Pass 0 for maxBytes to accept the
// package default.
func Serve(w http.ResponseWriter, r *http.Request, maxBytes int64, handler Handler) {
	logger := log.WithFunc("cocooncommon.admission.Serve")
	review, err := Decode(r, maxBytes)
	if err != nil {
		logger.Warnf(r.Context(), "decode admission review: %v", err)
		http.Error(w, "decode admission review", http.StatusBadRequest)
		return
	}
	resp := handler(r.Context(), review)
	if resp == nil {
		resp = Allow()
	}
	resp.UID = review.Request.UID
	review.Response = resp

	out, err := json.Marshal(review)
	if err != nil {
		logger.Error(r.Context(), err, "marshal admission review")
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out) //nolint:gosec // marshaled JSON API response, not rendered as HTML
}

// JSONPatchOp is a single RFC 6902 patch operation.
type JSONPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// MarshalPatch encodes a slice of JSONPatchOps as the []byte body an
// AdmissionResponse.Patch field expects.
func MarshalPatch(ops []JSONPatchOp) ([]byte, error) {
	out, err := json.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("marshal patch: %w", err)
	}
	return out, nil
}

// EscapeJSONPointer escapes the two characters that are reserved in
// RFC 6901 JSON Pointer paths so they can be safely embedded in a
// /metadata/annotations/<key> style patch path.
func EscapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
