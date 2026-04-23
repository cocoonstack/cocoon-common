package k8s

import (
	"cmp"
	"os"
	"strconv"
	"time"
)

// EnvOrDefault returns os.Getenv(key), falling back to fallback when unset or empty.
func EnvOrDefault(key, fallback string) string {
	return cmp.Or(os.Getenv(key), fallback)
}

// EnvDuration parses a duration env var, falling back to fallback when unset or invalid.
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

// EnvBool parses a boolean env var, falling back to fallback when unset or invalid.
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
