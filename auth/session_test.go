package auth

import (
	"testing"
	"time"
)

func TestSignAndVerifySession(t *testing.T) {
	t.Parallel()

	key := []byte("test-secret-key-32-bytes-long!!!")
	sess := Session{User: "alice", Email: "alice@example.com", Exp: time.Now().Add(time.Hour).Unix()}

	cookie := SignSession(sess, key)
	if cookie == "" {
		t.Fatal("SignSession returned empty string")
	}

	got, ok := VerifySession(cookie, key)
	if !ok {
		t.Fatal("VerifySession failed on valid cookie")
	}
	if got.User != sess.User || got.Email != sess.Email || got.Exp != sess.Exp {
		t.Errorf("session mismatch: got %+v, want %+v", got, sess)
	}
}

func TestVerifySessionRejectsTampered(t *testing.T) {
	t.Parallel()

	key := []byte("test-secret")
	cookie := SignSession(Session{User: "bob", Exp: time.Now().Add(time.Hour).Unix()}, key)

	if _, ok := VerifySession(cookie+"x", key); ok {
		t.Error("expected tampered cookie to fail")
	}
	if _, ok := VerifySession("garbage", key); ok {
		t.Error("expected garbage to fail")
	}
	if _, ok := VerifySession("", key); ok {
		t.Error("expected empty to fail")
	}
}

func TestVerifySessionWrongKey(t *testing.T) {
	t.Parallel()

	cookie := SignSession(Session{User: "carol"}, []byte("key-a"))
	if _, ok := VerifySession(cookie, []byte("key-b")); ok {
		t.Error("expected wrong key to fail")
	}
}

func TestRandomState(t *testing.T) {
	t.Parallel()

	s1 := RandomState()
	s2 := RandomState()
	if len(s1) != 32 {
		t.Errorf("RandomState length = %d, want 32", len(s1))
	}
	if s1 == s2 {
		t.Error("two RandomState calls returned the same value")
	}
}
