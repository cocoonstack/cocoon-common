package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomState returns a cryptographically random 32-character hex string
// suitable for OAuth state parameters and CSRF nonces.
func RandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) //nolint:errcheck // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b)
}
