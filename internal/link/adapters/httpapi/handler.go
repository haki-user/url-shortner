package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"tinyurl/internal/link/application"
	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type Handler struct {
	createGeneratedLink application.CreateGeneratedLink
	redirectLink        application.RedirectLink
	baseURL             string
}

func NewHandler(
	createGeneratedLink application.CreateGeneratedLink,
	redirectLink application.RedirectLink,
	baseURL string,
) Handler {
	return Handler{
		createGeneratedLink: createGeneratedLink,
		redirectLink:        redirectLink,
		baseURL:             baseURL,
	}
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
	code := strings.TrimPrefix(r.URL.Path, "/")

	result, err := h.redirectLink.Execute(r.Context(), application.RedirectLinkRequest{
		Code: code,
	})
	if err != nil {
		h.writeRedirectErr(w, err)
		return
	}

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
