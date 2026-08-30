package k8s

import (
	"context"
	"testing"
	"time"
)

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("COCOON_TEST_VAR", "set")
	if got := EnvOrDefault("COCOON_TEST_VAR", "fallback"); got != "set" {
		t.Errorf("set: got %q", got)
	}
	t.Setenv("COCOON_TEST_VAR", "")
	if got := EnvOrDefault("COCOON_TEST_VAR", "fallback"); got != "fallback" {
		t.Errorf("empty fallback: got %q", got)
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("COCOON_TEST_BOOL", "true")
	if !EnvBool("COCOON_TEST_BOOL", false) {
		t.Errorf("expected true")
	}
	t.Setenv("COCOON_TEST_BOOL", "garbage")
	if !EnvBool("COCOON_TEST_BOOL", true) {
		t.Errorf("bad input should fall back to true")
	}
	t.Setenv("COCOON_TEST_BOOL", "")
	if EnvBool("COCOON_TEST_BOOL", false) {
		t.Errorf("empty should fall back to false")
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("COCOON_TEST_INT", "42")
	if got := EnvInt("COCOON_TEST_INT", 7); got != 42 {
		t.Errorf("set: got %d, want 42", got)
	}
	t.Setenv("COCOON_TEST_INT", "garbage")
	if got := EnvInt("COCOON_TEST_INT", 7); got != 7 {
		t.Errorf("bad input: got %d, want fallback 7", got)
	}
	t.Setenv("COCOON_TEST_INT", "")
	if got := EnvInt("COCOON_TEST_INT", 7); got != 7 {
		t.Errorf("empty: got %d, want fallback 7", got)
	}
}

func TestSleepCtxReturnsTrueOnTimer(t *testing.T) {
	if !SleepCtx(t.Context(), time.Millisecond) {
		t.Errorf("timer path should return true")
	}
}

func TestSleepCtxReturnsFalseOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if SleepCtx(ctx, time.Hour) {
		t.Errorf("cancel path should return false")
	}
}

func TestSleepCtxZeroDurationReturnsImmediately(t *testing.T) {
	if !SleepCtx(t.Context(), 0) {
		t.Errorf("zero duration should return true without waiting")
	}
}
