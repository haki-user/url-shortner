package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Storage != StorageMemory {
		t.Fatalf("expected storage %q, got %q", StorageMemory, cfg.Storage)
	}

	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty database URL, got %q", cfg.DatabaseURL)
	}

	if cfg.Addr != ":8080" {
		t.Fatalf("expected addr %q, got %q", ":8080", cfg.Addr)
	}

	if cfg.BaseURL != "http://localhost:8080" {
		t.Fatalf("expected base URL %q, got %q", "http://localhost:8080", cfg.BaseURL)
	}

	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("expected shutdown timeout %v, got %v", 10*time.Second, cfg.ShutdownTimeout)
	}

	if cfg.DiagnosticsToken != "" {
		t.Fatalf("expected empty diagnostics token, got %q", cfg.DiagnosticsToken)
	}
}

func TestLoadPostgresWithDatabaseURL(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_STORAGE", "postgres")
	t.Setenv("TINYURL_DATABASE_URL", "postgres://tinyurl:tinyurl@localhost:5433/tinyurl?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Storage != StoragePostgres {
		t.Fatalf("expected storage %q, got %q", StoragePostgres, cfg.Storage)
	}

	if cfg.DatabaseURL != "postgres://tinyurl:tinyurl@localhost:5433/tinyurl?sslmode=disable" {
		t.Fatalf("unexpected database URL %q", cfg.DatabaseURL)
	}
}

func TestLoadPostgresWithoutDatabaseURLReturnsError(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_STORAGE", "postgres")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "TINYURL_DATABASE_URL is required") {
		t.Fatalf("expected database URL error, got %v", err)
	}
}

func TestLoadRedisCacheConfiguration(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_CACHE", "redis")
	t.Setenv("TINYURL_REDIS_URL", "redis://localhost:6379")
	t.Setenv("TINYURL_CACHE_OPERATION_TIMEOUT", "40ms")
	t.Setenv("TINYURL_CACHE_ACTIVE_TTL", "5m")
	t.Setenv("TINYURL_CACHE_INACTIVE_TTL", "45s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Cache != CacheRedis {
		t.Fatalf("expected cache %q, got %q", CacheRedis, cfg.Cache)
	}

	if cfg.RedisURL != "redis://localhost:6379" {
		t.Fatalf("unexpected Redis URL %q", cfg.RedisURL)
	}

	if cfg.CacheOperationTimeout != 40*time.Millisecond {
		t.Fatalf("expected operation timeout 40ms, got %s", cfg.CacheOperationTimeout)
	}

	if cfg.CacheActiveTTL != 5*time.Minute {
		t.Fatalf("expected active TTL 5m, got %s", cfg.CacheActiveTTL)
	}

	if cfg.CacheInactiveTTL != 45*time.Second {
		t.Fatalf("expected inactive TTL 45s, got %s", cfg.CacheInactiveTTL)
	}
}

func TestLoadNormalizesStorageCaseAndWhitespace(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_STORAGE", "  POSTGRES  ")
	t.Setenv("TINYURL_DATABASE_URL", "postgres://tinyurl:tinyurl@localhost:5433/tinyurl?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Storage != StoragePostgres {
		t.Fatalf("expected storage %q, got %q", StoragePostgres, cfg.Storage)
	}
}

func TestLoadUnsupportedStorageReturnsError(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_STORAGE", "sqlite")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid TINYURL_STORAGE") {
		t.Fatalf("expected invalid storage error, got %v", err)
	}
}

func TestLoadCustomAddressAndBaseURL(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_ADDR", ":9090")
	t.Setenv("TINYURL_BASE_URL", "https://tiny.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Addr != ":9090" {
		t.Fatalf("expected addr %q, got %q", ":9090", cfg.Addr)
	}

	if cfg.BaseURL != "https://tiny.example.com" {
		t.Fatalf("expected base URL %q, got %q", "https://tiny.example.com", cfg.BaseURL)
	}
}

func TestLoadDiagnosticsToken(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_DIAGNOSTICS_TOKEN", "  secret-token  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.DiagnosticsToken != "secret-token" {
		t.Fatalf("expected trimmed diagnostics token, got %q", cfg.DiagnosticsToken)
	}
}

func TestLoadRemovesTrailingSlashFromBaseURL(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_BASE_URL", "https://tiny.example.com/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.BaseURL != "https://tiny.example.com" {
		t.Fatalf("expected base URL %q, got %q", "https://tiny.example.com", cfg.BaseURL)
	}
}

func TestLoadInvalidBaseURLReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "not absolute", baseURL: "localhost:8080"},
		{name: "unsupported scheme", baseURL: "ftp://example.com"},
		{name: "missing host", baseURL: "https:///path"},
		{name: "parse error", baseURL: "://bad"},
		{name: "query", baseURL: "https://example.com?source=test"},
		{name: "fragment", baseURL: "https://example.com#section"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)

			t.Setenv("TINYURL_BASE_URL", tt.baseURL)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestLoadValidCustomShutdownTimeout(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_SHUTDOWN_TIMEOUT", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("expected shutdown timeout %v, got %v", 30*time.Second, cfg.ShutdownTimeout)
	}
}

func TestLoadInvalidShutdownTimeoutReturnsError(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("TINYURL_SHUTDOWN_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), `invalid TINYURL_SHUTDOWN_TIMEOUT "not-a-duration"`) {
		t.Fatalf("expected invalid shutdown timeout error with raw value, got %v", err)
	}
}

func TestLoadNonPositiveShutdownTimeoutReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
	}{
		{name: "zero", timeout: "0s"},
		{name: "negative", timeout: "-1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)

			t.Setenv("TINYURL_SHUTDOWN_TIMEOUT", tt.timeout)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), "TINYURL_SHUTDOWN_TIMEOUT must be positive") {
				t.Fatalf("expected positive timeout error, got %v", err)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	t.Setenv("TINYURL_STORAGE", "")
	t.Setenv("TINYURL_DATABASE_URL", "")
	t.Setenv("TINYURL_CACHE", "")
	t.Setenv("TINYURL_REDIS_URL", "")
	t.Setenv("TINYURL_CACHE_OPERATION_TIMEOUT", "")
	t.Setenv("TINYURL_CACHE_ACTIVE_TTL", "")
	t.Setenv("TINYURL_CACHE_INACTIVE_TTL", "")
	t.Setenv("TINYURL_ADDR", "")
	t.Setenv("TINYURL_BASE_URL", "")
	t.Setenv("TINYURL_SHUTDOWN_TIMEOUT", "")
	t.Setenv("TINYURL_DIAGNOSTICS_TOKEN", "")
}
