package domain

import (
	"errors"
	"testing"
)

func TestDestinationURLZeroValueIsZero(t *testing.T) {
	destination := DestinationURL{}

	if !destination.IsZero() {
		t.Fatal("expected zero-value DestinationURL to be zero")
	}
}

func TestDestinationURLZeroValueStringIsEmpty(t *testing.T) {
	destination := DestinationURL{}

	if destination.String() != "" {
		t.Fatalf("expected empty string, got %q", destination.String())
	}
}

func TestDestinationURLNotZeroValueIsNotZero(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}

	if destination.IsZero() {
		t.Fatal("expected non-zero-value DestinationURL to not be zero")
	}
}

func TestDestinationURLValueReturnsValue(t *testing.T) {
	value := "https://example.com"

	destination := DestinationURL{value: value}

	if destination.String() != value {
		t.Fatalf("expected %q, got %q", value, destination.String())
	}
}

func TestNewDestinationURLAcceptsValidDestinations(t *testing.T) {
	tests :=
		[]struct {
			name     string
			raw      string
			expected string
		}{
			{name: "http URL", raw: "http://example.com", expected: "http://example.com"},
			{name: "https URL", raw: "https://example.com", expected: "https://example.com"},
			{name: "trims surrounding whitespace", raw: " https://example.com ", expected: "https://example.com"},
			{name: "URL with path", raw: "https://example.com/path", expected: "https://example.com/path"},
			{name: "URL with query string", raw: "https://example.com/search?q=tinyurl", expected: "https://example.com/search?q=tinyurl"},
			{name: "URL with explicit port", raw: "https://example.com:8080/path", expected: "https://example.com:8080/path"},
			{name: "URL with query and fragment", raw: "https://example.com/search?q=tinyurl#section", expected: "https://example.com/search?q=tinyurl#section"},
			{name: "URL with uppercase scheme and hostname", raw: "HTTPS://EXAMPLE.COM/PATH", expected: "https://example.com/PATH"},
			{name: "normalization preserves explicit port", raw: "HTTPS://EXAMPLE.COM:8080/path", expected: "https://example.com:8080/path"},
			{name: "normalization preserves path casing", raw: "HTTPS://EXAMPLE.COM/Some/CaseSensitive/Path", expected: "https://example.com/Some/CaseSensitive/Path"},
			{name: "normalization preserves query and fragment", raw: "HTTPS://EXAMPLE.COM/search?q=TinyURL#Section", expected: "https://example.com/search?q=TinyURL#Section"},
			{name: "normalization preserves trailing slash", raw: "HTTPS://EXAMPLE.COM/", expected: "https://example.com/"},
		}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination, err := NewDestinationURL(tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if destination.String() != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, destination.String())
			}

			if destination.IsZero() {
				t.Fatal("expected destination to not be zero")
			}
		})
	}
}

func TestNewDestinationURLRejectsInvalidDestinations(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		expectedErr error
	}{
		{name: "empty string", raw: "", expectedErr: ErrEmptyDestination},
		{name: "whitespace only", raw: "   ", expectedErr: ErrEmptyDestination},
		{name: "malformed URL", raw: "://example.com", expectedErr: ErrMalformedDestination},
		{name: "malformed URL with space", raw: "https://ex ample.com", expectedErr: ErrMalformedDestination},
		{name: "unsupported ftp scheme", raw: "ftp://example.com", expectedErr: ErrUnsupportedScheme},
		{name: "unsupported mailto scheme", raw: "mailto:user@example.com", expectedErr: ErrUnsupportedScheme},
		{name: "missing scheme", raw: "example.com/path", expectedErr: ErrMalformedDestination},
		{name: "http URL missing host", raw: "http:/example.com", expectedErr: ErrMissingHost},
		{name: "https URL missing host", raw: "https:///path", expectedErr: ErrMissingHost},
		{name: "relative path", raw: "/relative/path", expectedErr: ErrMalformedDestination},
		{name: "javascript scheme", raw: "javascript:alert(1)", expectedErr: ErrUnsupportedScheme},
		{name: "file scheme", raw: "file:///tmp/data", expectedErr: ErrUnsupportedScheme},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination, err := NewDestinationURL(tt.raw)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if !destination.IsZero() {
				t.Fatal("expected destination to be zero")
			}
		})
	}
}
