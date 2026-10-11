// Package config reads service configuration from environment variables.
// Nothing (ports, hosts, credentials, timeouts) is hard-coded in the services.
package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Get returns the variable's value or the fallback when it is unset.
func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// MustGet returns the variable's value and aborts start-up if it is missing,
// so a misconfigured container fails loudly instead of misbehaving later.
func MustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

// Int parses an integer variable, falling back to a default when unset.
func Int(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("environment variable %s must be an integer, got %q", key, v)
	}
	return n
}

// Duration parses a Go duration variable such as "2s" or "200ms".
func Duration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("environment variable %s must be a duration like 2s, got %q", key, v)
	}
	return d
}
