package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewRedirectEventCreatesEvent(t *testing.T) {
	occurredAt := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)

	event, err := NewRedirectEvent(
		"abc123",
		occurredAt,
		"Mozilla/5.0",
		"https://referer.example.com",
		"127.0.0.1",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if event.Code != "abc123" {
		t.Fatalf("expected code %q, got %q", "abc123", event.Code)
	}

	if !event.OccurredAt.Equal(occurredAt) {
		t.Fatalf("expected occurredAt %v, got %v", occurredAt, event.OccurredAt)
	}

	if event.UserAgent != "Mozilla/5.0" {
		t.Fatalf("expected user agent %q, got %q", "Mozilla/5.0", event.UserAgent)
	}

	if event.Referer != "https://referer.example.com" {
		t.Fatalf("expected referer %q, got %q", "https://referer.example.com", event.Referer)
	}

	if event.IP != "127.0.0.1" {
		t.Fatalf("expected IP %q, got %q", "127.0.0.1", event.IP)
	}
}

func TestNewRedirectEventRejectsEmptyCode(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{name: "empty code", code: ""},
		{name: "whitespace-only code", code: "   "},
	}

	occurredAt := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := NewRedirectEvent(tt.code, occurredAt, "", "", "")
			if !errors.Is(err, ErrEmptyCode) {
				t.Fatalf("expected error %v, got %v", ErrEmptyCode, err)
			}

			if event != (RedirectEvent{}) {
				t.Fatalf("expected zero-value event, got %v", event)
			}
		})
	}
}

func TestNewRedirectEventRejectsZeroOccurredAt(t *testing.T) {
	event, err := NewRedirectEvent("abc123", time.Time{}, "", "", "")
	if !errors.Is(err, ErrZeroOccurredAt) {
		t.Fatalf("expected error %v, got %v", ErrZeroOccurredAt, err)
	}

	if event != (RedirectEvent{}) {
		t.Fatalf("expected zero-value event, got %v", event)
	}
}
