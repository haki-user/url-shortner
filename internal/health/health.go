package health

import (
	"context"
	"encoding/json"
	"net/http"
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

type Handler struct {
	readiness Checker
}

func NewHandler(readiness Checker) Handler {
	return Handler{
		readiness: readiness,
	}
}

type response struct {
	Status string `json:"status"`
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

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(value)
}
