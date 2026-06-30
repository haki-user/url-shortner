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

type CacheMode string

const (
	CacheNone  CacheMode = "none"
	CacheRedis CacheMode = "redis"
)

type Config struct {
	Storage               StorageMode
	DatabaseURL           string
	Cache                 CacheMode
	RedisURL              string
	CacheOperationTimeout time.Duration
	CacheActiveTTL        time.Duration
	CacheInactiveTTL      time.Duration
	Addr                  string
	BaseURL               string
	ShutdownTimeout       time.Duration
	DiagnosticsToken      string
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

	diagnosticsToken := strings.TrimSpace(os.Getenv("TINYURL_DIAGNOSTICS_TOKEN"))

	cache := CacheMode(strings.ToLower(strings.TrimSpace(
		os.Getenv("TINYURL_CACHE"),
	)))
	if cache == "" {
		cache = CacheNone
	}

	if cache != CacheNone && cache != CacheRedis {
		return Config{}, fmt.Errorf("invalid TINYURL_CACHE %q", cache)
	}

	redisURL := strings.TrimSpace(os.Getenv("TINYURL_REDIS_URL"))
	if cache == CacheRedis && redisURL == "" {
		return Config{}, fmt.Errorf(
			"TINYURL_REDIS_URL is required when TINYURL_CACHE=redis",
		)
	}

	cacheOperationTimeout, err := loadPositiveDuration(
		"TINYURL_CACHE_OPERATION_TIMEOUT",
		"25ms",
	)
	if err != nil {
		return Config{}, err
	}

	cacheActiveTTL, err := loadPositiveDuration(
		"TINYURL_CACHE_ACTIVE_TTL",
		"60s",
	)
	if err != nil {
		return Config{}, err
	}

	cacheInactiveTTL, err := loadPositiveDuration(
		"TINYURL_CACHE_INACTIVE_TTL",
		"30s",
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Storage:               storage,
		DatabaseURL:           databaseURL,
		Cache:                 cache,
		RedisURL:              redisURL,
		CacheOperationTimeout: cacheOperationTimeout,
		CacheActiveTTL:        cacheActiveTTL,
		CacheInactiveTTL:      cacheInactiveTTL,
		Addr:                  addr,
		BaseURL:               baseURL,
		ShutdownTimeout:       shutdownTimeout,
		DiagnosticsToken:      diagnosticsToken,
	}, nil
}

func loadPositiveDuration(name string, defaultValue string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		raw = defaultValue
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, raw, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}

	return value, nil
}
