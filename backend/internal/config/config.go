// Package config reads runtime settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds everything the server needs to start.
type Config struct {
	Port              string
	DatabaseURL       string
	CORSOrigins       []string
	APIKey            string // empty means API key auth is disabled
	LocationRateLimit int    // max location updates per IP per minute
}

// Load builds a Config from the environment, applying defaults that work
// for local development.
func Load() (Config, error) {
	cfg := Config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/geofence?sslmode=disable"),
		CORSOrigins: splitList(envOr("CORS_ORIGINS", "*")),
		APIKey:      strings.TrimSpace(os.Getenv("API_KEY")),
	}

	limit, err := strconv.Atoi(envOr("LOCATION_RATE_LIMIT", "120"))
	if err != nil || limit <= 0 {
		return Config{}, fmt.Errorf("LOCATION_RATE_LIMIT must be a positive integer, got %q", os.Getenv("LOCATION_RATE_LIMIT"))
	}
	cfg.LocationRateLimit = limit

	return cfg, nil
}

// AllowsAnyOrigin reports whether CORS is open to every origin.
func (c Config) AllowsAnyOrigin() bool {
	for _, o := range c.CORSOrigins {
		if o == "*" {
			return true
		}
	}
	return false
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimRight(strings.TrimSpace(part), "/"); p != "" {
			out = append(out, p)
		}
	}
	return out
}
