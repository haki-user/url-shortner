package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmptyCode      = errors.New("empty analytics code")
	ErrZeroOccurredAt = errors.New("zero analytics occurred at")
)

type RedirectEvent struct {
	Code       string
	OccurredAt time.Time
	UserAgent  string
	Referer    string
	IP         string
}

func NewRedirectEvent(
	code string,
	occurredAt time.Time,
	userAgent string,
	referer string,
	ip string,
) (RedirectEvent, error) {
	if strings.TrimSpace(code) == "" {
		return RedirectEvent{}, ErrEmptyCode
	}

	if occurredAt.IsZero() {
		return RedirectEvent{}, ErrZeroOccurredAt
	}

	return RedirectEvent{
		Code:       code,
		OccurredAt: occurredAt,
		UserAgent:  userAgent,
		Referer:    referer,
		IP:         ip,
	}, nil
}
