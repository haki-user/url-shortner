package application

import (
	"context"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
)

func TestRedirectCacheRefresherWritesUpdatedMapping(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	cache := &cacheAsideCacheFake{}

	refresher, err := NewRedirectCacheRefresher(
		cache,
		cacheAsideClockFake{now: now},
		RedirectCacheConfig{
			OperationTimeout: 25 * time.Millisecond,
			ActiveTTL:        60 * time.Second,
			InactiveTTL:      30 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("create refresher: %v", err)
	}

	destination, err := domain.NewDestinationURL("https://updated.example")
	if err != nil {
		t.Fatalf("create destination: %v", err)
	}

	link, err := domain.NewLink(
		"abc123",
		destination,
		"owner-1",
		now.Add(-time.Hour),
		nil,
	)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := link.Disable(now); err != nil {
		t.Fatalf("disable link: %v", err)
	}

	if err := refresher.Refresh(context.Background(), link); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	if cache.putCalls != 1 {
		t.Fatalf("expected one cache write, got %d", cache.putCalls)
	}

	if cache.putMapping.Version() != 2 {
		t.Fatalf("expected cached version 2, got %d", cache.putMapping.Version())
	}

	if cache.putMapping.Status() != domain.Disabled {
		t.Fatalf("expected disabled mapping, got %s", cache.putMapping.Status())
	}

	if cache.putTTL != 30*time.Second {
		t.Fatalf("expected inactive TTL 30s, got %s", cache.putTTL)
	}
}
