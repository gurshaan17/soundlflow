package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration for the API server.
type Config struct {
	Port            int
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

const (
	defaultPort            = 8080
	defaultShutdownTimeout = 10 * time.Second
)

// defaultDatabaseURL is only used when neither DATABASE_URL nor a .env file
// provides one. It matches the docker-compose.yml local Postgres.
const defaultDatabaseURL = "postgres://soundflow:soundflow@localhost:5432/soundflow?sslmode=disable"

// Load reads configuration from environment variables, falling back to a
// .env file in the working directory and finally to local defaults.
func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:            envInt("PORT", defaultPort),
		DatabaseURL:     envString("DATABASE_URL", defaultDatabaseURL),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("config: PORT must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	return cfg, nil
}

func envString(key, fallback string) string {
	if v := osGetenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := osGetenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := osGetenv(key)
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}

var osGetenv = os.Getenv
