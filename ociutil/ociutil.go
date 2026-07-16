// Package ociutil provides shared helpers for OCI blobs, digests, and refs.
package ociutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// SHA256Hex returns the hex-encoded SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// CopyBlobExact copies exactly size bytes and verifies both length and sha256 digest.
func CopyBlobExact(dst io.Writer, body io.Reader, digest string, size int64) error {
	h := sha256.New()
	written, err := io.CopyN(io.MultiWriter(dst, h), body, size)
	if err != nil {
		return fmt.Errorf("copy blob %s: %w", digest, err)
	}
	if extra, _ := io.Copy(io.Discard, body); extra > 0 {
		return fmt.Errorf("blob %s longer than manifest size %d (got %d extra)", digest, size, extra)
	}
	if written != size {
		return fmt.Errorf("blob %s shorter than manifest size %d (got %d)", digest, size, written)
	}
	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	want := digest
	if !strings.HasPrefix(want, "sha256:") {
		want = "sha256:" + want
	}
	if got != want {
		return fmt.Errorf("blob %s digest mismatch: got %s", digest, got)
	}
	return nil
}

// ParseRef splits "name:tag" into name and tag; defaults tag to "latest".
func ParseRef(ref string) (string, string) {
	if name, tag, ok := strings.Cut(ref, ":"); ok && name != "" {
		return name, tag
	}
	return ref, "latest"
}
