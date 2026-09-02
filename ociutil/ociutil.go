// Package ociutil provides shared helpers for OCI blobs, digests, and refs.
package ociutil

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	relRepo = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*$`)
	relTag  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
)

// BlobSizeChecker enforces exact size and no trailing data on a digest-verified body.
type BlobSizeChecker struct {
	body   io.Reader
	digest string
	size   int64
	lim    io.LimitedReader
	done   bool
}

// NewBlobSizeChecker wraps body so that reads fail on a short or long blob.
func NewBlobSizeChecker(body io.Reader, digest string, size int64) *BlobSizeChecker {
	return &BlobSizeChecker{body: body, digest: digest, size: size, lim: io.LimitedReader{R: body, N: size}}
}

func (v *BlobSizeChecker) Read(p []byte) (int, error) {
	if v.done {
		return 0, io.EOF
	}
	n, err := v.lim.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	if !errors.Is(err, io.EOF) {
		return n, nil
	}
	if finErr := v.finish(); finErr != nil {
		return n, finErr
	}
	v.done = true
	if n > 0 {
		return n, nil
	}
	return 0, io.EOF
}

func (v *BlobSizeChecker) finish() error {
	if v.lim.N > 0 {
		return fmt.Errorf("blob %s shorter than manifest size %d (missing %d)", v.digest, v.size, v.lim.N)
	}
	var probe [1]byte
	extra, err := v.body.Read(probe[:])
	if extra > 0 {
		return fmt.Errorf("blob %s longer than manifest size %d", v.digest, v.size)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("verify blob %s: %w", v.digest, err)
	}
	return nil
}

// SHA256Hex returns the hex-encoded SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// CopyBlobSized copies exactly size bytes from a digest-verified body.
func CopyBlobSized(dst io.Writer, body io.Reader, digest string, size int64) error {
	_, err := io.Copy(dst, NewBlobSizeChecker(body, digest, size))
	return err
}

// ParseRef splits a registry-relative "repo[:tag]" at its first colon, defaulting the tag to "latest".
func ParseRef(ref string) (string, string) {
	if name, tag, ok := strings.Cut(ref, ":"); ok && name != "" {
		return name, tag
	}
	return ref, "latest"
}

// IsRelativeRef reports whether ref is a registry-relative repo[:tag], the only form ParseRef splits correctly.
func IsRelativeRef(ref string) bool {
	repo, tag := ParseRef(ref)
	return relRepo.MatchString(repo) && relTag.MatchString(tag)
}
