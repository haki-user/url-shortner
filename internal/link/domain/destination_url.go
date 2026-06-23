package domain

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrEmptyDestination     = errors.New("empty destination")
	ErrMalformedDestination = errors.New("malformed destination")
	ErrUnsupportedScheme    = errors.New("unsupported scheme")
	ErrMissingHost          = errors.New("missing host")
)

type DestinationURL struct {
	value string
}

func NewDestinationURL(raw string) (DestinationURL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DestinationURL{}, ErrEmptyDestination
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return DestinationURL{}, ErrMalformedDestination
	}

	if parsed.Scheme == "" {
		return DestinationURL{}, ErrMalformedDestination
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return DestinationURL{}, ErrUnsupportedScheme
	}

	if parsed.Host == "" {
		return DestinationURL{}, ErrMissingHost
	}

	parsed.Host = strings.ToLower(parsed.Host)

	return DestinationURL{value: parsed.String()}, nil
}

func (d DestinationURL) String() string {
	return d.value
}

func (d DestinationURL) IsZero() bool {
	return d.value == ""
}
