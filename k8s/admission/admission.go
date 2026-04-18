// Package admission provides shared admission-webhook helpers: decode/dispatch/encode, allow/deny builders, and JSON patch utilities.
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

// DefaultMaxBody is the request-body ceiling Serve applies when the caller passes 0.
const DefaultMaxBody int64 = 10 << 20

// Handler is the admission callback. A nil return is treated as Allow().
type Handler func(ctx context.Context, review *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// Allow returns an AdmissionResponse that permits the request.
func Allow() *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{Allowed: true}
}

// Deny returns a forbidden AdmissionResponse with the given message.
func Deny(msg string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
			Reason:  metav1.StatusReasonForbidden,
		},
	}
}

// Decode parses an AdmissionReview from r, using DefaultMaxBody when maxBytes <= 0.
func Decode(r *http.Request, maxBytes int64) (*admissionv1.AdmissionReview, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBody
	}
	review := &admissionv1.AdmissionReview{}
	return review, json.NewDecoder(io.LimitReader(r.Body, maxBytes)).Decode(review)
}

// Serve decodes an AdmissionReview, dispatches to handler, and writes the response.
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
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
