package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// RandomState returns a cryptographically random 32-character hex string
// suitable for OAuth state parameters and CSRF nonces.
// Panics on crypto/rand failure — a weak nonce silently breaks CSRF.
func RandomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read: %v", err))
	}
	return hex.EncodeToString(b)
}
