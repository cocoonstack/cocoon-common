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
	return envParse(key, fallback, strconv.ParseBool)
}

func envParse[T any](key string, fallback T, parse func(string) (T, error)) T {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	t, err := parse(v)
	if err != nil {
		return fallback
	}
	return t
}
