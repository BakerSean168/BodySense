package config

import (
	"os"
	"strings"
)

// EnvOr returns the trimmed value of the environment variable named by key,
// or fallback when the variable is unset or blank after trimming. It is the
// single shared implementation for the leaf config loaders (dr,
// uploadstorage) that read the environment outside the composition root.
func EnvOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
