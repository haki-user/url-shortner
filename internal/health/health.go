package health

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const readinessTimeout = 2 * time.Second

type Checker interface {
	Check(context.Context) error
}

type CheckerFunc func(context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

type ComponentCheck struct {
	Name     string
	Required bool
	Checker  Checker
}

type Handler struct {
	readiness        Checker
	diagnostics      []ComponentCheck
	diagnosticsToken string
}

func NewHandler(
	readiness Checker,
	diagnostics []ComponentCheck,
	diagnosticsToken string,
) Handler {
	return Handler{
		readiness:        readiness,
		diagnostics:      diagnostics,
		diagnosticsToken: diagnosticsToken,
	}
}

type response struct {
	Status string `json:"status"`
}

type diagnosticsResponse struct {
	Status string                             `json:"status"`
	Checks map[string]diagnosticCheckResponse `json:"checks"`
}

type diagnosticCheckResponse struct {
	Status    string `json:"status"`
	Required  bool   `json:"required"`
	LatencyMS int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

func (Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{
		Status: "ok",
	})
}

func (h Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	if h.readiness == nil {
		writeJSON(w, http.StatusServiceUnavailable, response{
			Status: "not_ready",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	if err := h.readiness.Check(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, response{
			Status: "not_ready",
		})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Status: "ok",
	})
}

func (h Handler) Diagnostics(w http.ResponseWriter, r *http.Request) {
	if h.diagnosticsToken == "" {
		writeJSON(w, http.StatusNotFound, response{
			Status: "not_found",
		})
		return
	}

	if !authorized(r, h.diagnosticsToken) {
		writeJSON(w, http.StatusUnauthorized, response{
			Status: "unauthorized",
		})
		return
	}

	checks := make(map[string]diagnosticCheckResponse, len(h.diagnostics))
	overallStatus := "ok"
	statusCode := http.StatusOK

	for _, check := range h.diagnostics {
		result := runComponentCheck(r.Context(), check)
		checks[check.Name] = result

		if result.Status == "ok" {
			continue
		}

		if result.Required {
			overallStatus = "not_ready"
			statusCode = http.StatusServiceUnavailable
			continue
		}

		if overallStatus == "ok" {
			overallStatus = "degraded"
		}
	}

	writeJSON(w, statusCode, diagnosticsResponse{
		Status: overallStatus,
		Checks: checks,
	})
}

func runComponentCheck(
	parent context.Context,
	check ComponentCheck,
) diagnosticCheckResponse {
	startedAt := time.Now()

	ctx, cancel := context.WithTimeout(parent, readinessTimeout)
	defer cancel()

	result := diagnosticCheckResponse{
		Status:   "ok",
		Required: check.Required,
	}

	if check.Checker == nil {
		result.Status = "error"
		result.Error = "checker is not configured"
		result.LatencyMS = time.Since(startedAt).Milliseconds()
		return result
	}

	if err := check.Checker.Check(ctx); err != nil {
		result.Status = "error"
		result.Error = err.Error()
	}

	result.LatencyMS = time.Since(startedAt).Milliseconds()
	return result
}

func authorized(r *http.Request, expected string) bool {
	if tokenMatches(r.Header.Get("X-Diagnostics-Token"), expected) {
		return true
	}

	if tokenMatches(r.Header.Get("X-Admin-Token"), expected) {
		return true
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	bearerToken, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok {
		return false
	}

	return tokenMatches(strings.TrimSpace(bearerToken), expected)
}

func tokenMatches(actual string, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}

	actualHash := sha256.Sum256([]byte(actual))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) == 1
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(value)
}
