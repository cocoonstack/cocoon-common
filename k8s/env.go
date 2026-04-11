package k8s

import (
	"context"
	"os"
	"strconv"
	"time"
)

// EnvOrDefault returns the value of key from the environment, or
// fallback when key is unset or empty. Empty is intentional — a
// deployment that exports VAR="" is usually a misconfiguration and
// we prefer the documented default to a silent empty value.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// EnvDuration parses a duration env var. An unset or unparseable
// value falls back to the supplied default so a binary stays
// bootable when an operator typoes the override.
func EnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// EnvBool parses a boolean env var. Anything strconv.ParseBool
// rejects (including an empty string) falls back to the supplied
// default so a binary stays bootable when an operator typoes the
// override.
func EnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// SleepCtx blocks for d or until ctx is canceled, returning false
// when the context fires first so callers can exit their retry loop
// without a second select. Zero or negative d returns immediately
// with true (no wait requested).
func SleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
