package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// RandomState returns a cryptographically random 32-character hex string
// suitable for OAuth state parameters and CSRF nonces.
//
// Panics if crypto/rand.Read fails. On supported platforms the syscall
// is documented to never fail; a failure indicates the OS RNG is
// unavailable, in which case continuing with a weak nonce would silently
// compromise CSRF protection.
func RandomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read: %v", err))
	}
	return hex.EncodeToString(b)
}
