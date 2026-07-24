package k8s

import (
	"cmp"
	"os"
	"strconv"
)

// EnvOrDefault returns os.Getenv(key), falling back to fallback when unset or empty.
func EnvOrDefault(key, fallback string) string {
	return cmp.Or(os.Getenv(key), fallback)
}

// EnvBool parses a boolean env var, falling back to fallback when unset or invalid.
func EnvBool(key string, fallback bool) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return b
}
