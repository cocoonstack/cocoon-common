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

func TestEnvDuration(t *testing.T) {
	t.Setenv("COCOON_TEST_DUR", "5s")
	if got := EnvDuration("COCOON_TEST_DUR", time.Second); got != 5*time.Second {
		t.Errorf("parsed: %v", got)
	}
	t.Setenv("COCOON_TEST_DUR", "not-a-dur")
	if got := EnvDuration("COCOON_TEST_DUR", 2*time.Second); got != 2*time.Second {
		t.Errorf("bad input fallback: %v", got)
	}
	t.Setenv("COCOON_TEST_DUR", "")
	if got := EnvDuration("COCOON_TEST_DUR", 3*time.Second); got != 3*time.Second {
		t.Errorf("empty fallback: %v", got)
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
