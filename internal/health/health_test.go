package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessReturnsOKWhenRequiredCheckerPasses(t *testing.T) {
	handler := NewHandler(
		CheckerFunc(func(context.Context) error {
			return nil
		}),
		nil,
		"",
	)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	handler.Readiness(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	assertBodyStatus(t, response.Body.String(), "ok")
}

func TestReadinessReturnsNotReadyWhenRequiredCheckerFails(t *testing.T) {
	handler := NewHandler(
		CheckerFunc(func(context.Context) error {
			return errors.New("database down")
		}),
		nil,
		"",
	)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	handler.Readiness(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
	}

	assertBodyStatus(t, response.Body.String(), "not_ready")
}

func TestDiagnosticsIsDisabledWithoutToken(t *testing.T) {
	handler := NewHandler(nil, nil, "")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics", nil)

	handler.Diagnostics(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestDiagnosticsRequiresToken(t *testing.T) {
	handler := NewHandler(nil, nil, "secret")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics", nil)

	handler.Diagnostics(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestDiagnosticsReturnsDegradedWhenOptionalCheckFails(t *testing.T) {
	handler := NewHandler(
		nil,
		[]ComponentCheck{
			{
				Name:     "postgres",
				Required: true,
				Checker: CheckerFunc(func(context.Context) error {
					return nil
				}),
			},
			{
				Name:     "redis",
				Required: false,
				Checker: CheckerFunc(func(context.Context) error {
					return errors.New("redis refused connection")
				}),
			},
		},
		"secret",
	)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics", nil)
	request.Header.Set("Authorization", "Bearer secret")

	handler.Diagnostics(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	payload := decodeDiagnosticsResponse(t, response.Body.String())
	if payload.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", payload.Status)
	}

	redis := payload.Checks["redis"]
	if redis.Status != "error" {
		t.Fatalf("expected redis error, got %q", redis.Status)
	}

	if redis.Required {
		t.Fatal("expected redis to be optional")
	}

	if !strings.Contains(redis.Error, "redis refused connection") {
		t.Fatalf("expected redis error in response, got %q", redis.Error)
	}
}

func TestDiagnosticsReturnsNotReadyWhenRequiredCheckFails(t *testing.T) {
	handler := NewHandler(
		nil,
		[]ComponentCheck{
			{
				Name:     "postgres",
				Required: true,
				Checker: CheckerFunc(func(context.Context) error {
					return errors.New("postgres timeout")
				}),
			},
			{
				Name:     "redis",
				Required: false,
				Checker: CheckerFunc(func(context.Context) error {
					return nil
				}),
			},
		},
		"secret",
	)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics", nil)
	request.Header.Set("X-Diagnostics-Token", "secret")

	handler.Diagnostics(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
	}

	payload := decodeDiagnosticsResponse(t, response.Body.String())
	if payload.Status != "not_ready" {
		t.Fatalf("expected not_ready status, got %q", payload.Status)
	}

	postgres := payload.Checks["postgres"]
	if !postgres.Required {
		t.Fatal("expected postgres to be required")
	}
}

func assertBodyStatus(t *testing.T, body string, expected string) {
	t.Helper()

	var payload response
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Status != expected {
		t.Fatalf("expected status %q, got %q", expected, payload.Status)
	}
}

func decodeDiagnosticsResponse(t *testing.T, body string) diagnosticsResponse {
	t.Helper()

	var payload diagnosticsResponse
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode diagnostics response: %v", err)
	}

	return payload
}
