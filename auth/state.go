package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomState returns a cryptographically random 32-character hex string for OAuth state parameters and CSRF nonces.
func RandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails per the Go 1.24+ crypto/rand contract
	return hex.EncodeToString(b)
}
