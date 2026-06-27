package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tinyurl/internal/link/application"
	"tinyurl/internal/link/ports"
)

type ManagementHandler struct {
	getManagedLink application.GetManagedLink
}

func NewManagementHandler(getManagedLink application.GetManagedLink) ManagementHandler {
	return ManagementHandler{getManagedLink: getManagedLink}
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

	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, link.Version()))

	writeJSON(w, http.StatusOK, managedLinkResponse{
		Code:        link.Code(),
		Destination: link.Destination().String(),
		Status:      link.Status().String(),
		CreatedAt:   link.CreatedAt(),
		UpdatedAt:   link.UpdatedAt(),
		ExpiresAt:   link.ExpiresAt(),
		Version:     link.Version(),
	})
}
