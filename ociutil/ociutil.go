// Package ociutil provides shared helpers for OCI blobs, digests, and refs.
package ociutil

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"regexp"
	"strings"
)

var (
	relRepo = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*$`)
	relTag  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
)

// SHA256Hex returns the hex-encoded SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// CopyBlobExact copies exactly size bytes and verifies both length and sha256 digest.
func CopyBlobExact(dst io.Writer, body io.Reader, digest string, size int64) error {
	_, err := io.Copy(dst, NewBlobVerifier(body, digest, size))
	return err
}

// BlobVerifier wraps a blob body and enforces the manifest contract while it
// is read: exactly size bytes, no trailing data, matching sha256 digest. Read
// returns io.EOF only after every check passed; violations surface as errors.
type BlobVerifier struct {
	body   io.Reader
	digest string
	size   int64
	lim    io.LimitedReader
	hash   hash.Hash
	done   bool
}

func NewBlobVerifier(body io.Reader, digest string, size int64) *BlobVerifier {
	v := &BlobVerifier{body: body, digest: digest, size: size, hash: sha256.New()}
	v.lim = io.LimitedReader{R: io.TeeReader(body, v.hash), N: size}
	return v
}

func (v *BlobVerifier) Read(p []byte) (int, error) {
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

func (v *BlobVerifier) finish() error {
	if v.lim.N > 0 {
		return fmt.Errorf("blob %s shorter than manifest size %d (missing %d)", v.digest, v.size, v.lim.N)
	}
	var probe [1]byte
	if extra, _ := v.body.Read(probe[:]); extra > 0 {
		return fmt.Errorf("blob %s longer than manifest size %d", v.digest, v.size)
	}
	got := "sha256:" + hex.EncodeToString(v.hash.Sum(nil))
	want := v.digest
	if !strings.HasPrefix(want, "sha256:") {
		want = "sha256:" + want
	}
	if got != want {
		return fmt.Errorf("blob %s digest mismatch: got %s", v.digest, got)
	}
	return nil
}

// ParseRef splits a registry-relative "repo[:tag]" at its first colon,
// defaulting tag to "latest"; IsRelativeRef guards the domain.
func ParseRef(ref string) (string, string) {
	if name, tag, ok := strings.Cut(ref, ":"); ok && name != "" {
		return name, tag
	}
	return ref, "latest"
}

// IsRelativeRef reports whether ref is a registry-relative repo[:tag], the
// only form ParseRef splits correctly (ports and digests carry extra colons).
func IsRelativeRef(ref string) bool {
	repo, tag := ParseRef(ref)
	return relRepo.MatchString(repo) && relTag.MatchString(tag)
}
