// Package auth provides shared HMAC-signed session helpers used by
// glance and epoch for SSO cookie management.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Session holds the claims embedded in an HMAC-signed cookie.
type Session struct {
	User  string `json:"u"`
	Email string `json:"e"`
	Exp   int64  `json:"x"` // unix timestamp
}

// SignSession encodes and HMAC-signs a session into a cookie value.
func SignSession(sess Session, key []byte) (string, error) {
	data, err := json.Marshal(sess)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

// VerifySession validates the HMAC signature and decodes the session.
// Exp == 0 means "no expiry".
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
	sess := &Session{}
	if json.Unmarshal(data, sess) != nil {
		return nil, false
	}
	if sess.Exp != 0 && sess.Exp < time.Now().Unix() {
		return nil, false
	}
	return sess, true
}
