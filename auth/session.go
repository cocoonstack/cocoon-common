// Package auth provides shared HMAC-signed session helpers used by
// glance and epoch for SSO cookie management.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// Session holds the claims embedded in an HMAC-signed cookie.
type Session struct {
	User  string `json:"u"`
	Email string `json:"e"`
	Exp   int64  `json:"x"` // unix timestamp
}

// SignSession encodes and HMAC-signs a session into a cookie value.
func SignSession(sess Session, key []byte) string {
	data, _ := json.Marshal(sess) //nolint:errcheck // Session fields are always serializable
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// VerifySession validates the HMAC signature and decodes the session.
// Returns nil and false if the signature is invalid or decoding fails.
func VerifySession(cookie string, key []byte) (*Session, bool) {
	payload, sig, ok := strings.Cut(cookie, ".")
	if !ok {
		return nil, false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return nil, false
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, false
	}
	var sess Session
	if json.Unmarshal(data, &sess) != nil {
		return nil, false
	}
	return &sess, true
}

// RandomState returns a cryptographically random 32-character hex string
// suitable for OAuth state parameters and CSRF nonces.
func RandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) //nolint:errcheck // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b)
}
