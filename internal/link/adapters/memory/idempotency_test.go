package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

func TestIdempotencyStoreGetMissingReturnsNotFound(t *testing.T) {
	store := NewIdempotencyStore()

	link, err := store.Get(context.Background(), "owner-1", "key-1")
	if !errors.Is(err, ports.ErrIdempotencyKeyNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrIdempotencyKeyNotFound, err)
	}

	if !memoryIdempotencyLinkIsZero(link) {
		t.Fatal("expected zero-value link")
	}
}

func TestIdempotencyStoreSaveThenGetReturnsStoredLink(t *testing.T) {
	store := NewIdempotencyStore()
	link := mustNewIdempotencyTestLink(t, "abc123", "https://example.com")

	err := store.Save(context.Background(), "owner-1", "key-1", link)
	if err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	stored, err := store.Get(context.Background(), "owner-1", "key-1")
	if err != nil {
		t.Fatalf("expected get to succeed, got %v", err)
	}

	if stored != link {
		t.Fatalf("expected stored link %v, got %v", link, stored)
	}
}

func TestIdempotencyStoreSaveDuplicateSameLinkReturnsNil(t *testing.T) {
	store := NewIdempotencyStore()
	link := mustNewIdempotencyTestLink(t, "abc123", "https://example.com")

	err := store.Save(context.Background(), "owner-1", "key-1", link)
	if err != nil {
		t.Fatalf("expected first save to succeed, got %v", err)
	}

	err = store.Save(context.Background(), "owner-1", "key-1", link)
	if err != nil {
		t.Fatalf("expected duplicate same-link save to succeed, got %v", err)
	}
}

func TestIdempotencyStoreSaveDuplicateDifferentLinkReturnsConflict(t *testing.T) {
	store := NewIdempotencyStore()

	original := mustNewIdempotencyTestLink(t, "abc123", "https://example.com")
	different := mustNewIdempotencyTestLink(t, "def456", "https://other.example.com")

	err := store.Save(context.Background(), "owner-1", "key-1", original)
	if err != nil {
		t.Fatalf("expected first save to succeed, got %v", err)
	}

	err = store.Save(context.Background(), "owner-1", "key-1", different)
	if !errors.Is(err, ports.ErrIdempotencyKeyConflict) {
		t.Fatalf("expected error %v, got %v", ports.ErrIdempotencyKeyConflict, err)
	}

	stored, err := store.Get(context.Background(), "owner-1", "key-1")
	if err != nil {
		t.Fatalf("expected get to succeed, got %v", err)
	}

	if stored != original {
		t.Fatalf("expected original link to remain stored, got %v", stored)
	}
}

func TestIdempotencyStoreAllowsSameKeyForDifferentOwners(t *testing.T) {
	store := NewIdempotencyStore()

	ownerOneLink := mustNewIdempotencyTestLink(t, "abc123", "https://owner-one.example.com")
	ownerTwoLink := mustNewIdempotencyTestLink(t, "def456", "https://owner-two.example.com")

	err := store.Save(context.Background(), "owner-1", "same-key", ownerOneLink)
	if err != nil {
		t.Fatalf("expected owner one save to succeed, got %v", err)
	}

	err = store.Save(context.Background(), "owner-2", "same-key", ownerTwoLink)
	if err != nil {
		t.Fatalf("expected owner two save to succeed, got %v", err)
	}

	storedOwnerOneLink, err := store.Get(context.Background(), "owner-1", "same-key")
	if err != nil {
		t.Fatalf("expected owner one get to succeed, got %v", err)
	}

	storedOwnerTwoLink, err := store.Get(context.Background(), "owner-2", "same-key")
	if err != nil {
		t.Fatalf("expected owner two get to succeed, got %v", err)
	}

	if storedOwnerOneLink != ownerOneLink {
		t.Fatalf("expected owner one link %v, got %v", ownerOneLink, storedOwnerOneLink)
	}

	if storedOwnerTwoLink != ownerTwoLink {
		t.Fatalf("expected owner two link %v, got %v", ownerTwoLink, storedOwnerTwoLink)
	}
}

func TestIdempotencyStoreGetCanceledContextReturnsContextCanceled(t *testing.T) {
	store := NewIdempotencyStore()
	link := mustNewIdempotencyTestLink(t, "abc123", "https://example.com")

	err := store.Save(context.Background(), "owner-1", "key-1", link)
	if err != nil {
		t.Fatalf("expected setup save to succeed, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stored, err := store.Get(ctx, "owner-1", "key-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	if !memoryIdempotencyLinkIsZero(stored) {
		t.Fatal("expected zero-value link")
	}
}

func TestIdempotencyStoreSaveCanceledContextReturnsContextCanceled(t *testing.T) {
	store := NewIdempotencyStore()
	link := mustNewIdempotencyTestLink(t, "abc123", "https://example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Save(ctx, "owner-1", "key-1", link)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	stored, err := store.Get(context.Background(), "owner-1", "key-1")
	if !errors.Is(err, ports.ErrIdempotencyKeyNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrIdempotencyKeyNotFound, err)
	}

	if !memoryIdempotencyLinkIsZero(stored) {
		t.Fatal("expected zero-value link")
	}
}

func mustNewIdempotencyTestLink(t *testing.T, code string, rawDestination string) domain.Link {
	t.Helper()

	destination, err := domain.NewDestinationURL(rawDestination)
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	createdAt := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	link, err := domain.NewLink(code, destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	return link
}

func memoryIdempotencyLinkIsZero(link domain.Link) bool {
	return link.Code() == "" &&
		link.Destination().IsZero() &&
		link.OwnerID() == "" &&
		link.Status() == domain.Unknown &&
		link.CreatedAt().IsZero() &&
		link.UpdatedAt().IsZero() &&
		link.ExpiresAt() == nil &&
		link.Version() == 0
}
