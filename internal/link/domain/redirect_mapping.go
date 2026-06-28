package domain

import (
	"strings"
	"time"
)

// RedirectMapping contains only the domain data required to decide a redirect.
type RedirectMapping struct {
	code        string
	destination DestinationURL
	status      LinkStatus
	expiresAt   *time.Time
	version     uint64
}

func NewRedirectMapping(
	code string,
	destination DestinationURL,
	status LinkStatus,
	expiresAt *time.Time,
	version uint64,
) (RedirectMapping, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return RedirectMapping{}, ErrEmptyCode
	}

	if destination.IsZero() {
		return RedirectMapping{}, ErrZeroDestination
	}

	if !status.IsValid() {
		return RedirectMapping{}, ErrInvalidLinkStatus
	}

	if version == 0 {
		return RedirectMapping{}, ErrZeroVersion
	}

	return RedirectMapping{
		code:        code,
		destination: destination,
		status:      status,
		expiresAt:   copyTimePointer(expiresAt),
		version:     version,
	}, nil
}

func RedirectMappingFromLink(link Link) (RedirectMapping, error) {
	return NewRedirectMapping(
		link.Code(),
		link.Destination(),
		link.Status(),
		link.ExpiresAt(),
		link.Version(),
	)
}

func (m RedirectMapping) Code() string {
	return m.code
}

func (m RedirectMapping) Destination() DestinationURL {
	return m.destination
}

func (m RedirectMapping) Status() LinkStatus {
	return m.status
}

func (m RedirectMapping) ExpiresAt() *time.Time {
	return copyTimePointer(m.expiresAt)
}

func (m RedirectMapping) Version() uint64 {
	return m.version
}

func (m RedirectMapping) IsExpired(now time.Time) bool {
	if m.expiresAt == nil {
		return false
	}

	return !now.Before(*m.expiresAt)
}

func (m RedirectMapping) CanRedirect(now time.Time) bool {
	if now.IsZero() {
		return false
	}

	return m.status.CanRedirect() && !m.IsExpired(now)
}

func copyTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}
