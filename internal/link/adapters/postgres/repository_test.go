package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
	storagepostgres "tinyurl/internal/storage/postgres"
)

func TestRepositoryInsertStoresLink(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link := mustNewRepositoryTestLink(t, "insert001")

	err := repository.Insert(ctx, link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	var count int
	err = repository.pool.QueryRow(
		ctx,
		`select count(*) from links where code = $1`,
		link.Code(),
	).Scan(&count)
	if err != nil {
		t.Fatalf("expected count query to succeed, got %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestRepositoryInsertDuplicateReturnsAlreadyExists(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link := mustNewRepositoryTestLink(t, "dupe001")

	err := repository.Insert(ctx, link)
	if err != nil {
		t.Fatalf("expected first insert to succeed, got %v", err)
	}

	err = repository.Insert(ctx, link)
	if !errors.Is(err, ports.ErrLinkAlreadyExists) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkAlreadyExists, err)
	}
}

func TestRepositoryFindByCodeReturnsStoredLink(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link := mustNewRepositoryTestLink(t, "find001")

	if err := repository.Insert(ctx, link); err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	stored, err := repository.FindByCode(ctx, link.Code())
	if err != nil {
		t.Fatalf("expected find to succeed, got %v", err)
	}

	assertRepositoryLinksEqual(t, stored, link)
}

func TestRepositoryFindByCodeRestoresPersistedLifecycleState(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	destination, err := domain.NewDestinationURL("https://example.com/lifecycle001")
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	createdAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)

	link, err := domain.NewLink("life001", destination, "owner-1", createdAt, &expiresAt)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	if err := link.Disable(createdAt.Add(time.Hour)); err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	if err := repository.Insert(ctx, link); err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	stored, err := repository.FindByCode(ctx, link.Code())
	if err != nil {
		t.Fatalf("expected find to succeed, got %v", err)
	}

	assertRepositoryLinksEqual(t, stored, link)
}

func TestRepositoryFindByCodeMissingReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link, err := repository.FindByCode(ctx, "missing")
	if !errors.Is(err, ports.ErrLinkNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkNotFound, err)
	}

	if !repositoryLinkIsZero(link) {
		t.Fatalf("expected zero-value link, got %v", link)
	}
}

func TestRepositoryUpdateWithMatchingVersionUpdatesLink(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link := mustNewRepositoryTestLink(t, "update001")

	if err := repository.Insert(ctx, link); err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	newDestination, err := domain.NewDestinationURL("https://updated.example.com")
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	updatedAt := link.CreatedAt().Add(time.Hour)
	if err := link.UpdateDestination(newDestination, updatedAt); err != nil {
		t.Fatalf("expected domain update to succeed, got %v", err)
	}

	err = repository.Update(ctx, link, 1)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	stored, err := repository.FindByCode(ctx, link.Code())
	if err != nil {
		t.Fatalf("expected find to succeed, got %v", err)
	}

	assertRepositoryLinksEqual(t, stored, link)
}

func TestRepositoryUpdateWithMismatchedVersionReturnsVersionConflict(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link := mustNewRepositoryTestLink(t, "conflict001")

	if err := repository.Insert(ctx, link); err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	newDestination, err := domain.NewDestinationURL("https://updated.example.com")
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	if err := link.UpdateDestination(newDestination, link.CreatedAt().Add(time.Hour)); err != nil {
		t.Fatalf("expected domain update to succeed, got %v", err)
	}

	err = repository.Update(ctx, link, 999)
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("expected error %v, got %v", ports.ErrVersionConflict, err)
	}
}

func TestRepositoryUpdateMissingCodeReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t, ctx)
	cleanupLinks(t, ctx, repository)

	link := mustNewRepositoryTestLink(t, "missing001")

	err := repository.Update(ctx, link, 1)
	if !errors.Is(err, ports.ErrLinkNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkNotFound, err)
	}
}

func newTestRepository(t *testing.T, ctx context.Context) *Repository {
	t.Helper()

	databaseURL := os.Getenv("TINYURL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TINYURL_TEST_DATABASE_URL not set")
	}

	pool, err := storagepostgres.OpenPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("expected pool to open, got %v", err)
	}

	t.Cleanup(pool.Close)

	return NewRepository(pool)
}

func cleanupLinks(t *testing.T, ctx context.Context, repository *Repository) {
	t.Helper()

	_, err := repository.pool.Exec(ctx, `delete from redirect_events`)
	if err != nil {
		t.Fatalf("expected redirect_events cleanup to succeed, got %v", err)
	}

	_, err = repository.pool.Exec(ctx, `delete from idempotency_keys`)
	if err != nil {
		t.Fatalf("expected idempotency_keys cleanup to succeed, got %v", err)
	}

	_, err = repository.pool.Exec(ctx, `delete from links`)
	if err != nil {
		t.Fatalf("expected links cleanup to succeed, got %v", err)
	}
}

func mustNewRepositoryTestLink(t *testing.T, code string) domain.Link {
	t.Helper()

	destination, err := domain.NewDestinationURL("https://example.com/" + code)
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	createdAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)

	link, err := domain.NewLink(code, destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	return link
}

func assertRepositoryLinksEqual(t *testing.T, actual domain.Link, expected domain.Link) {
	t.Helper()

	if actual.Code() != expected.Code() {
		t.Fatalf("expected code %q, got %q", expected.Code(), actual.Code())
	}

	if actual.Destination() != expected.Destination() {
		t.Fatalf("expected destination %q, got %q", expected.Destination(), actual.Destination())
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

	actualExpiresAt := actual.ExpiresAt()
	expectedExpiresAt := expected.ExpiresAt()

	if expectedExpiresAt == nil {
		if actualExpiresAt != nil {
			t.Fatalf("expected nil expiresAt, got %v", actualExpiresAt)
		}
	} else {
		if actualExpiresAt == nil {
			t.Fatal("expected expiresAt, got nil")
		}

		if !actualExpiresAt.Equal(*expectedExpiresAt) {
			t.Fatalf("expected expiresAt %v, got %v", *expectedExpiresAt, *actualExpiresAt)
		}
	}

	if actual.Version() != expected.Version() {
		t.Fatalf("expected version %d, got %d", expected.Version(), actual.Version())
	}
}

func repositoryLinkIsZero(link domain.Link) bool {
	return link.Code() == "" &&
		link.Destination().IsZero() &&
		link.OwnerID() == "" &&
		link.Status() == domain.Unknown &&
		link.CreatedAt().IsZero() &&
		link.UpdatedAt().IsZero() &&
		link.ExpiresAt() == nil &&
		link.Version() == 0
}
