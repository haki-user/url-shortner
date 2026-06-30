package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	analyticsdomain "tinyurl/internal/analytics/domain"
	analyticsports "tinyurl/internal/analytics/ports"
	"tinyurl/internal/link/application"
	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type Handler struct {
	createGeneratedLink application.CreateGeneratedLink
	redirectLink        application.RedirectLink
	baseURL             string
	analyticsRecorder   analyticsports.RedirectEventRecorder
	clock               ports.Clock
	metrics             MetricsRecorder
}

type HandlerOption func(*Handler)

type MetricsRecorder interface {
	RecordRedirect(result string, duration time.Duration)
	RecordAnalytics(result string, duration time.Duration)
}

func WithAnalytics(
	recorder analyticsports.RedirectEventRecorder,
	clock ports.Clock,
) HandlerOption {
	return func(h *Handler) {
		h.analyticsRecorder = recorder
		h.clock = clock
	}
}

func WithMetrics(recorder MetricsRecorder) HandlerOption {
	return func(h *Handler) {
		h.metrics = recorder
	}
}

func NewHandler(
	createGeneratedLink application.CreateGeneratedLink,
	redirectLink application.RedirectLink,
	baseURL string,
	options ...HandlerOption,
) Handler {
	handler := Handler{
		createGeneratedLink: createGeneratedLink,
		redirectLink:        redirectLink,
		baseURL:             baseURL,
	}

	for _, option := range options {
		option(&handler)
	}

	return handler
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/links" {
		h.handleCreateLink(w, r)
		return
	}

	if r.Method == http.MethodGet && isCodePath(r.URL.Path) {
		h.handleRedirect(w, r)
		return
	}

	http.NotFound(w, r)
}

type createLinkHTTPRequest struct {
	Destination string     `json:"destination"`
	OwnerID     string     `json:"ownerId"`
	ExpiresAt   *time.Time `json:"expiresAt"`
}

type createLinkHTTPResponse struct {
	Code        string `json:"code"`
	ShortURL    string `json:"shortUrl"`
	Destination string `json:"destination"`
}

func (h Handler) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var request createLinkHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	link, err := h.createGeneratedLink.Execute(r.Context(), application.CreateGeneratedLinkRequest{
		Destination:    request.Destination,
		OwnerID:        request.OwnerID,
		ExpiresAt:      request.ExpiresAt,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		h.writeCreateError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createLinkHTTPResponse{
		Code:        link.Code(),
		ShortURL:    h.baseURL + "/" + link.Code(),
		Destination: link.Destination().String(),
	})
}

func (h Handler) handleRedirect(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	resultName := "error"
	defer func() {
		h.recordRedirectMetric(resultName, time.Since(startedAt))
	}()

	code := strings.TrimPrefix(r.URL.Path, "/")

	result, err := h.redirectLink.Execute(r.Context(), application.RedirectLinkRequest{
		Code: code,
	})
	if err != nil {
		if errors.Is(err, ports.ErrLinkNotFound) || errors.Is(err, application.ErrLinkUnavailable) {
			resultName = "not_found"
		}

		h.writeRedirectErr(w, err)
		return
	}

	h.recordRedirectEvent(r, code)

	resultName = "success"
	http.Redirect(w, r, result.Destination, http.StatusFound)
}

func (h Handler) writeCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrLinkAlreadyExists):
		http.Error(w, "conflict", http.StatusConflict)
	case isValidationError(err):
		http.Error(w, "bad request", http.StatusBadRequest)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (h Handler) writeRedirectErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrLinkNotFound), errors.Is(err, application.ErrLinkUnavailable):
		http.NotFound(w, nil)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func isValidationError(err error) bool {
	return errors.Is(err, domain.ErrEmptyDestination) ||
		errors.Is(err, domain.ErrMalformedDestination) ||
		errors.Is(err, domain.ErrUnsupportedScheme) ||
		errors.Is(err, domain.ErrMissingHost) ||
		errors.Is(err, domain.ErrEmptyCode) ||
		errors.Is(err, domain.ErrZeroDestination) ||
		errors.Is(err, domain.ErrEmptyOwnerID) ||
		errors.Is(err, domain.ErrZeroCreatedAt) ||
		errors.Is(err, domain.ErrInvalidExpiresAt)
}

func isCodePath(path string) bool {
	if path == "" || path == "/" {
		return false
	}

	if !strings.HasPrefix(path, "/") {
		return false
	}

	return !strings.Contains(strings.TrimPrefix(path, "/"), "/")
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func (h Handler) recordRedirectEvent(r *http.Request, code string) {
	if h.analyticsRecorder == nil || h.clock == nil {
		h.recordAnalyticsMetric("skipped", 0)
		return
	}

	event, err := analyticsdomain.NewRedirectEvent(
		code,
		h.clock.Now(),
		r.UserAgent(),
		r.Referer(),
		clientIPFromRequest(r),
	)
	if err != nil {
		h.recordAnalyticsMetric("error", 0)
		return
	}

	startedAt := time.Now()
	if err := h.analyticsRecorder.Record(r.Context(), event); err != nil {
		h.recordAnalyticsMetric("error", time.Since(startedAt))
		return
	}

	h.recordAnalyticsMetric("success", time.Since(startedAt))
}

func (h Handler) recordRedirectMetric(result string, duration time.Duration) {
	if h.metrics == nil {
		return
	}

	h.metrics.RecordRedirect(result, duration)
}

func (h Handler) recordAnalyticsMetric(result string, duration time.Duration) {
	if h.metrics == nil {
		return
	}

	h.metrics.RecordAnalytics(result, duration)
}

func clientIPFromRequest(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		return strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
