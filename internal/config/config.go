package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type StorageMode string

const (
	StorageMemory   StorageMode = "memory"
	StoragePostgres StorageMode = "postgres"
)

type Config struct {
	Storage         StorageMode
	DatabaseURL     string
	Addr            string
	BaseURL         string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	storage := StorageMode(strings.ToLower(strings.TrimSpace(os.Getenv("TINYURL_STORAGE"))))
	if storage == "" {
		storage = StorageMemory
	}

	if storage != StorageMemory && storage != StoragePostgres {
		return Config{}, fmt.Errorf("invalid TINYURL_STORAGE %q", storage)
	}

	databaseURL := strings.TrimSpace(os.Getenv("TINYURL_DATABASE_URL"))
	if storage == StoragePostgres && databaseURL == "" {
		return Config{}, fmt.Errorf("TINYURL_DATABASE_URL is required when TINYURL_STORAGE=postgres")
	}

	addr := strings.TrimSpace(os.Getenv("TINYURL_ADDR"))
	if addr == "" {
		addr = ":8080"
	}

	baseURL := strings.TrimSpace(os.Getenv("TINYURL_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	baseURL = strings.TrimRight(baseURL, "/")

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid TINYURL_BASE_URL %q: %w", baseURL, err)
	}

	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return Config{}, fmt.Errorf("TINYURL_BASE_URL must use http or https")
	}

	if parsedBaseURL.Host == "" {
		return Config{}, fmt.Errorf("TINYURL_BASE_URL must be absolute URL")
	}

	if parsedBaseURL.RawQuery != "" || parsedBaseURL.Fragment != "" {
		return Config{}, fmt.Errorf("TINYURL_BASE_URL must not contain a query or fragment")
	}

	shutdownTimeoutRaw := strings.TrimSpace(os.Getenv("TINYURL_SHUTDOWN_TIMEOUT"))
	if shutdownTimeoutRaw == "" {
		shutdownTimeoutRaw = "10s"
	}

	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid TINYURL_SHUTDOWN_TIMEOUT %q: %w", shutdownTimeoutRaw, err)
	}

	if shutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("TINYURL_SHUTDOWN_TIMEOUT must be positive")
	}

	return Config{
		Storage:         storage,
		DatabaseURL:     databaseURL,
		Addr:            addr,
		BaseURL:         baseURL,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}
