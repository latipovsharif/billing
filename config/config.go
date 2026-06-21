package config

import "os"

// Config holds runtime configuration read from the environment.
type Config struct {
	DatabaseURL string
	HTTPAddr    string
	GraceDays   int // past_due -> suspended grace window
	PaymeURL    string
	PaymeMerch  string
	PaymeKey    string
	TokenEncKey string
	KaspiURL    string
	KaspiAPIKey string
	KaspiDevice string
	KaspiOrgBIN string
}

// Load reads config from env with sane defaults for local dev.
func Load() Config {
	return Config{
		DatabaseURL: getenv("BILLING_DATABASE_URL", "postgres://postgres:123@localhost:5432/billing?sslmode=disable"),
		HTTPAddr:    getenv("BILLING_HTTP_ADDR", "localhost:4000"),
		GraceDays:   3,
		PaymeURL:    os.Getenv("PAYME_SUBSCRIBE_URL"),
		PaymeMerch:  os.Getenv("PAYME_MERCHANT_ID"),
		PaymeKey:    os.Getenv("PAYME_SUBSCRIBE_KEY"),
		TokenEncKey: os.Getenv("PAYME_TOKEN_ENC_KEY"),
		KaspiURL:    os.Getenv("KASPI_BASE_URL"),
		KaspiAPIKey: os.Getenv("KASPI_API_KEY"),
		KaspiDevice: os.Getenv("KASPI_DEVICE_TOKEN"),
		KaspiOrgBIN: os.Getenv("KASPI_ORG_BIN"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
