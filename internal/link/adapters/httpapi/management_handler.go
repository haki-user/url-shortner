package httpapi

import (
	"bytes"
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
	getManagedLink        application.GetManagedLink
	changeLinkStatus      application.ChangeLinkStatus
	changeLinkDestination application.ChangeLinkDestination
	changeLinkExpiration  application.ChangeLinkExpiration
}

func NewManagementHandler(
	getManagedLink application.GetManagedLink,
	changeLinkStatus application.ChangeLinkStatus,
	changeLinkDestination application.ChangeLinkDestination,
	changeLinkExpiration application.ChangeLinkExpiration,
) ManagementHandler {
	return ManagementHandler{
		getManagedLink:        getManagedLink,
		changeLinkStatus:      changeLinkStatus,
		changeLinkDestination: changeLinkDestination,
		changeLinkExpiration:  changeLinkExpiration,
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

type patchLinkHTTPRequest struct {
	Status      *string         `json:"status"`
	Destination *string         `json:"destination"`
	ExpiresAt   json.RawMessage `json:"expiresAt"`
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

	var request patchLinkHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	providedFields := 0
	if request.Status != nil {
		providedFields++
	}
	if request.Destination != nil {
		providedFields++
	}
	if request.ExpiresAt != nil {
		providedFields++
	}
	if providedFields != 1 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var link domain.Link

	switch {
	case request.Status != nil:
		status, parseErr := domain.ParseLinkStatus(
			strings.ToLower(strings.TrimSpace(*request.Status)),
		)
		if parseErr != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		link, err = h.changeLinkStatus.Execute(
			r.Context(),
			application.ChangeLinkStatusRequest{
				Code:            r.PathValue("code"),
				OwnerID:         ownerID,
				Status:          status,
				ExpectedVersion: expectedVersion,
			},
		)

	case request.Destination != nil:
		link, err = h.changeLinkDestination.Execute(
			r.Context(),
			application.ChangeLinkDestinationRequest{
				Code:            r.PathValue("code"),
				OwnerID:         ownerID,
				Destination:     *request.Destination,
				ExpectedVersion: expectedVersion,
			},
		)

	case request.ExpiresAt != nil:
		expiresAt, parseErr := parseExpiresAt(request.ExpiresAt)
		if parseErr != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		link, err = h.changeLinkExpiration.Execute(
			r.Context(),
			application.ChangeLinkExpirationRequest{
				Code:            r.PathValue("code"),
				OwnerID:         ownerID,
				ExpiresAt:       expiresAt,
				ExpectedVersion: expectedVersion,
			},
		)
	}

	if err != nil {
		writeManagementMutationError(w, r, err)
		return
	}

	writeManagedLinkResponse(w, http.StatusOK, link)
}

func writeManagementMutationError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, ports.ErrLinkNotFound),
		errors.Is(err, application.ErrLinkAccessDenied):
		http.NotFound(w, r)
	case errors.Is(err, ports.ErrVersionConflict):
		http.Error(w, "precondition failed", http.StatusPreconditionFailed)
	case errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrDeletedLink),
		errors.Is(err, domain.ErrUnchangedDestination),
		errors.Is(err, domain.ErrUnchangedExpiration),
		errors.Is(err, domain.ErrNoExpiration):
		http.Error(w, "conflict", http.StatusConflict)
	case errors.Is(err, application.ErrInvalidExpectedVersion),
		errors.Is(err, domain.ErrInvalidLinkStatus),
		isValidationError(err):
		http.Error(w, "bad request", http.StatusBadRequest)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func parseExpiresAt(value json.RawMessage) (*time.Time, error) {
	trimmed := bytes.TrimSpace(value)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	var expiresAt time.Time
	if err := json.Unmarshal(trimmed, &expiresAt); err != nil {
		return nil, err
	}

	return &expiresAt, nil
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
