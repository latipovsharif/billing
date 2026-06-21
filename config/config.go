package config

import "os"

// Config holds runtime configuration read from the environment.
type Config struct {
	DatabaseURL string
	HTTPAddr    string
	GraceDays   int // past_due -> suspended grace window
}

// Load reads config from env with sane defaults for local dev.
func Load() Config {
	return Config{
		DatabaseURL: getenv("BILLING_DATABASE_URL", "postgres://postgres:123@localhost:5432/billing?sslmode=disable"),
		HTTPAddr:    getenv("BILLING_HTTP_ADDR", "localhost:4000"),
		GraceDays:   3,
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
