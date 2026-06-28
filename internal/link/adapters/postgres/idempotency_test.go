package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
	storagepostgres "tinyurl/internal/storage/postgres"
)

func TestPostgresIdempotencyStoreGetMissingReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresIdempotencyTestPool(t, ctx)
	store := NewIdempotencyStore(pool)

	link, err := store.Get(ctx, "owner-1", "key-1")
	if !errors.Is(err, ports.ErrIdempotencyKeyNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrIdempotencyKeyNotFound, err)
	}

	if !postgresIdempotencyLinkIsZero(link) {
		t.Fatal("expected zero-value link")
	}
}

func TestPostgresIdempotencyStoreSaveThenGetReturnsOriginalLink(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresIdempotencyTestPool(t, ctx)

	repository := NewRepository(pool)
	store := NewIdempotencyStore(pool)

	link := mustNewPostgresIdempotencyTestLink(t, "abc123", "owner-1", "https://example.com")

	if err := repository.Insert(ctx, link); err != nil {
		t.Fatalf("expected link insert to succeed, got %v", err)
	}

	if err := store.Save(ctx, "owner-1", "key-1", link); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	stored, err := store.Get(ctx, "owner-1", "key-1")
	if err != nil {
		t.Fatalf("expected get to succeed, got %v", err)
	}

	assertPostgresIdempotencyLink(t, stored, link)
}

func TestPostgresIdempotencyStoreSaveSameOwnerKeySameLinkTwiceSucceeds(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresIdempotencyTestPool(t, ctx)

	repository := NewRepository(pool)
	store := NewIdempotencyStore(pool)

	link := mustNewPostgresIdempotencyTestLink(t, "abc123", "owner-1", "https://example.com")

	if err := repository.Insert(ctx, link); err != nil {
		t.Fatalf("expected link insert to succeed, got %v", err)
	}

	if err := store.Save(ctx, "owner-1", "key-1", link); err != nil {
		t.Fatalf("expected first save to succeed, got %v", err)
	}

	if err := store.Save(ctx, "owner-1", "key-1", link); err != nil {
		t.Fatalf("expected duplicate same-link save to succeed, got %v", err)
	}
}

func TestPostgresIdempotencyStoreSaveSameOwnerKeyDifferentLinkReturnsConflict(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresIdempotencyTestPool(t, ctx)

	repository := NewRepository(pool)
	store := NewIdempotencyStore(pool)

	original := mustNewPostgresIdempotencyTestLink(t, "abc123", "owner-1", "https://example.com")
	different := mustNewPostgresIdempotencyTestLink(t, "def456", "owner-1", "https://other.example.com")

	if err := repository.Insert(ctx, original); err != nil {
		t.Fatalf("expected original link insert to succeed, got %v", err)
	}

	if err := repository.Insert(ctx, different); err != nil {
		t.Fatalf("expected different link insert to succeed, got %v", err)
	}

	if err := store.Save(ctx, "owner-1", "key-1", original); err != nil {
		t.Fatalf("expected first save to succeed, got %v", err)
	}

	err := store.Save(ctx, "owner-1", "key-1", different)
	if !errors.Is(err, ports.ErrIdempotencyKeyConflict) {
		t.Fatalf("expected error %v, got %v", ports.ErrIdempotencyKeyConflict, err)
	}
}

func TestPostgresIdempotencyStoreAllowsSameKeyForDifferentOwners(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresIdempotencyTestPool(t, ctx)

	repository := NewRepository(pool)
	store := NewIdempotencyStore(pool)

	ownerOneLink := mustNewPostgresIdempotencyTestLink(t, "abc123", "owner-1", "https://owner-one.example.com")
	ownerTwoLink := mustNewPostgresIdempotencyTestLink(t, "def456", "owner-2", "https://owner-two.example.com")

	if err := repository.Insert(ctx, ownerOneLink); err != nil {
		t.Fatalf("expected owner one link insert to succeed, got %v", err)
	}

	if err := repository.Insert(ctx, ownerTwoLink); err != nil {
		t.Fatalf("expected owner two link insert to succeed, got %v", err)
	}

	if err := store.Save(ctx, "owner-1", "same-key", ownerOneLink); err != nil {
		t.Fatalf("expected owner one save to succeed, got %v", err)
	}

	if err := store.Save(ctx, "owner-2", "same-key", ownerTwoLink); err != nil {
		t.Fatalf("expected owner two save to succeed, got %v", err)
	}

	storedOwnerOneLink, err := store.Get(ctx, "owner-1", "same-key")
	if err != nil {
		t.Fatalf("expected owner one get to succeed, got %v", err)
	}

	storedOwnerTwoLink, err := store.Get(ctx, "owner-2", "same-key")
	if err != nil {
		t.Fatalf("expected owner two get to succeed, got %v", err)
	}

	assertPostgresIdempotencyLink(t, storedOwnerOneLink, ownerOneLink)
	assertPostgresIdempotencyLink(t, storedOwnerTwoLink, ownerTwoLink)
}

func TestPostgresIdempotencyStoreSaveMissingLinkFKReturnsDatabaseError(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresIdempotencyTestPool(t, ctx)

	store := NewIdempotencyStore(pool)
	link := mustNewPostgresIdempotencyTestLink(t, "missing-link", "owner-1", "https://example.com")

	err := store.Save(ctx, "owner-1", "key-1", link)
	if err == nil {
		t.Fatal("expected database error, got nil")
	}

	if errors.Is(err, ports.ErrIdempotencyKeyNotFound) {
		t.Fatalf("expected database error, got sentinel %v", ports.ErrIdempotencyKeyNotFound)
	}

	if errors.Is(err, ports.ErrIdempotencyKeyConflict) {
		t.Fatalf("expected database error, got sentinel %v", ports.ErrIdempotencyKeyConflict)
	}
}

func newPostgresIdempotencyTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TINYURL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TINYURL_TEST_DATABASE_URL is not set")
	}

	pool, err := storagepostgres.OpenPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("expected pool creation to succeed, got %v", err)
	}

	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `
		truncate table idempotency_keys, links restart identity cascade
	`)
	if err != nil {
		t.Fatalf("expected test tables truncate to succeed, got %v", err)
	}

	return pool
}

func mustNewPostgresIdempotencyTestLink(
	t *testing.T,
	code string,
	ownerID string,
	rawDestination string,
) domain.Link {
	t.Helper()

	destination, err := domain.NewDestinationURL(rawDestination)
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	createdAt := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)

	link, err := domain.NewLink(code, destination, ownerID, createdAt, nil)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	return link
}

func assertPostgresIdempotencyLink(t *testing.T, actual domain.Link, expected domain.Link) {
	t.Helper()

	if actual.Code() != expected.Code() {
		t.Fatalf("expected code %q, got %q", expected.Code(), actual.Code())
	}

	if actual.Destination().String() != expected.Destination().String() {
		t.Fatalf("expected destination %q, got %q", expected.Destination().String(), actual.Destination().String())
	}

	if actual.OwnerID() != expected.OwnerID() {
		t.Fatalf("expected ownerID %q, got %q", expected.OwnerID(), actual.OwnerID())
	}

	if actual.Status() != expected.Status() {
		t.Fatalf("expected status %v, got %v", expected.Status(), actual.Status())
	}

	if !actual.CreatedAt().Equal(expected.CreatedAt()) {
		t.Fatalf("expected createdAt %v, got %v", expected.CreatedAt(), actual.CreatedAt())
	}

	if !actual.UpdatedAt().Equal(expected.UpdatedAt()) {
		t.Fatalf("expected updatedAt %v, got %v", expected.UpdatedAt(), actual.UpdatedAt())
	}

	if actual.Version() != expected.Version() {
		t.Fatalf("expected version %d, got %d", expected.Version(), actual.Version())
	}

	actualExpiresAt := actual.ExpiresAt()
	expectedExpiresAt := expected.ExpiresAt()

	if expectedExpiresAt == nil {
		if actualExpiresAt != nil {
			t.Fatalf("expected nil expiresAt, got %v", actualExpiresAt)
		}

		return
	}

	if actualExpiresAt == nil {
		t.Fatal("expected expiresAt, got nil")
	}

	if !actualExpiresAt.Equal(*expectedExpiresAt) {
		t.Fatalf("expected expiresAt %v, got %v", *expectedExpiresAt, *actualExpiresAt)
	}
}

func postgresIdempotencyLinkIsZero(link domain.Link) bool {
	return link.Code() == "" &&
		link.Destination().IsZero() &&
		link.OwnerID() == "" &&
		link.Status() == domain.Unknown &&
		link.CreatedAt().IsZero() &&
		link.UpdatedAt().IsZero() &&
		link.ExpiresAt() == nil &&
		link.Version() == 0
}
