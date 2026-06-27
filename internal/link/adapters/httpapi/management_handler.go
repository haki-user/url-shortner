package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tinyurl/internal/link/application"
	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type ManagementHandler struct {
	getManagedLink   application.GetManagedLink
	changeLinkStatus application.ChangeLinkStatus
}

func NewManagementHandler(
	getManagedLink application.GetManagedLink,
	changeLinkStatus application.ChangeLinkStatus,
) ManagementHandler {
	return ManagementHandler{
		getManagedLink:   getManagedLink,
		changeLinkStatus: changeLinkStatus,
	}
}

type managedLinkResponse struct {
	Code        string     `json:"code"`
	Destination string     `json:"destination"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	Version     uint64     `json:"version"`
}

func (h ManagementHandler) Get(w http.ResponseWriter, r *http.Request) {
	ownerID := strings.TrimSpace(r.Header.Get("X-Owner-ID"))
	if ownerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	link, err := h.getManagedLink.Execute(r.Context(), application.GetManagedLinkRequest{
		Code:    r.PathValue("code"),
		OwnerID: ownerID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrLinkNotFound),
			errors.Is(err, application.ErrLinkAccessDenied):
			http.NotFound(w, r)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	writeManagedLinkResponse(w, http.StatusOK, link)
}

type changeLinkStatusHTTPRequest struct {
	Status string `json:"status"`
}

func (h ManagementHandler) Patch(w http.ResponseWriter, r *http.Request) {
	ownerID := strings.TrimSpace(r.Header.Get("X-Owner-ID"))
	if ownerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	if ifMatch == "" {
		http.Error(w, "If-Match header is required", http.StatusPreconditionRequired)
		return
	}

	expectedVersion, err := parseIfMatch(ifMatch)
	if err != nil {
		http.Error(w, "invalid If-Match header", http.StatusBadRequest)
		return
	}

	var request changeLinkStatusHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	status, err := domain.ParseLinkStatus(
		strings.ToLower(strings.TrimSpace(request.Status)),
	)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	link, err := h.changeLinkStatus.Execute(
		r.Context(),
		application.ChangeLinkStatusRequest{
			Code:            r.PathValue("code"),
			OwnerID:         ownerID,
			Status:          status,
			ExpectedVersion: expectedVersion,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrLinkNotFound),
			errors.Is(err, application.ErrLinkAccessDenied):
			http.NotFound(w, r)
		case errors.Is(err, ports.ErrVersionConflict):
			http.Error(w, "precondition failed", http.StatusPreconditionFailed)
		case errors.Is(err, domain.ErrInvalidTransition):
			http.Error(w, "conflict", http.StatusConflict)
		case errors.Is(err, application.ErrInvalidExpectedVersion),
			errors.Is(err, domain.ErrInvalidLinkStatus):
			http.Error(w, "bad request", http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	writeManagedLinkResponse(w, http.StatusOK, link)
}

func parseIfMatch(value string) (uint64, error) {
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return 0, application.ErrInvalidExpectedVersion
	}

	version, err := strconv.ParseUint(value[1:len(value)-1], 10, 64)
	if err != nil || version == 0 {
		return 0, application.ErrInvalidExpectedVersion
	}

	return version, nil
}

func writeManagedLinkResponse(
	w http.ResponseWriter,
	statusCode int,
	link domain.Link,
) {
	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, link.Version()))
	w.Header().Set("Cache-Control", "private, no-store")

	writeJSON(w, statusCode, managedLinkResponse{
		Code:        link.Code(),
		Destination: link.Destination().String(),
		Status:      link.Status().String(),
		CreatedAt:   link.CreatedAt(),
		UpdatedAt:   link.UpdatedAt(),
		ExpiresAt:   link.ExpiresAt(),
		Version:     link.Version(),
	})
}
