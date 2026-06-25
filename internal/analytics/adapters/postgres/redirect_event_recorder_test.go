package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticsdomain "tinyurl/internal/analytics/domain"
	linkpostgres "tinyurl/internal/link/adapters/postgres"
	linkdomain "tinyurl/internal/link/domain"
	storagepostgres "tinyurl/internal/storage/postgres"
)

func TestPostgresRedirectEventRecorderRecordStoresEvent(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresRedirectEventRecorderTestPool(t, ctx)

	link := mustNewPostgresRedirectEventRecorderTestLink(t, "abc123")
	linkRepository := linkpostgres.NewRepository(pool)

	if err := linkRepository.Insert(ctx, link); err != nil {
		t.Fatalf("expected link insert to succeed, got %v", err)
	}

	recorder := NewRedirectEventRecorder(pool)

	occurredAt := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)

	event, err := analyticsdomain.NewRedirectEvent(
		"abc123",
		occurredAt,
		"Mozilla/5.0",
		"https://referer.example.com",
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf("expected event setup to succeed, got %v", err)
	}

	if err := recorder.Record(ctx, event); err != nil {
		t.Fatalf("expected record to succeed, got %v", err)
	}

	var storedCode string
	var storedOccurredAt time.Time
	var storedUserAgent string
	var storedReferer string
	var storedIP string

	err = pool.QueryRow(ctx, `
		select code, occurred_at, user_agent, referer, ip
		from redirect_events
		where code = $1
	`, "abc123").Scan(
		&storedCode,
		&storedOccurredAt,
		&storedUserAgent,
		&storedReferer,
		&storedIP,
	)
	if err != nil {
		t.Fatalf("expected stored redirect event, got %v", err)
	}

	if storedCode != event.Code {
		t.Fatalf("expected code %q, got %q", event.Code, storedCode)
	}

	if !storedOccurredAt.Equal(event.OccurredAt) {
		t.Fatalf("expected occurredAt %v, got %v", event.OccurredAt, storedOccurredAt)
	}

	if storedUserAgent != event.UserAgent {
		t.Fatalf("expected userAgent %q, got %q", event.UserAgent, storedUserAgent)
	}

	if storedReferer != event.Referer {
		t.Fatalf("expected referer %q, got %q", event.Referer, storedReferer)
	}

	if storedIP != event.IP {
		t.Fatalf("expected IP %q, got %q", event.IP, storedIP)
	}
}

func TestPostgresRedirectEventRecorderRecordMissingLinkFKReturnsDatabaseError(t *testing.T) {
	ctx := context.Background()
	pool := newPostgresRedirectEventRecorderTestPool(t, ctx)

	recorder := NewRedirectEventRecorder(pool)

	event, err := analyticsdomain.NewRedirectEvent(
		"missing-link",
		time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC),
		"Mozilla/5.0",
		"https://referer.example.com",
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf("expected event setup to succeed, got %v", err)
	}

	err = recorder.Record(ctx, event)
	if err == nil {
		t.Fatal("expected database error, got nil")
	}
}

func newPostgresRedirectEventRecorderTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
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
		truncate table redirect_events, idempotency_keys, links restart identity cascade
	`)
	if err != nil {
		t.Fatalf("expected test tables truncate to succeed, got %v", err)
	}

	return pool
}

func mustNewPostgresRedirectEventRecorderTestLink(t *testing.T, code string) linkdomain.Link {
	t.Helper()

	destination, err := linkdomain.NewDestinationURL("https://example.com")
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	createdAt := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)

	link, err := linkdomain.NewLink(code, destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	return link
}
