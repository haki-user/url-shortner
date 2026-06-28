package application

import (
	"context"
	"testing"
	"time"

	"tinyurl/internal/link/adapters/memory"
	"tinyurl/internal/link/domain"
)

type createGeneratedLinkIntegrationGenerator struct {
	code string
}

func (g createGeneratedLinkIntegrationGenerator) Generate(ctx context.Context) (string, error) {
	return g.code, nil
}

type integrationClock struct {
	now time.Time
}

func (c integrationClock) Now() time.Time {
	return c.now
}

func TestCreateGeneratedLinkWithMemoryRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := memory.NewRepository()
	generator := createGeneratedLinkIntegrationGenerator{code: "abc123"}
	clock := integrationClock{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "HTTPS://EXAMPLE.COM/Path?q=TinyURL#Section",
		OwnerID:     "owner-1",
		ExpiresAt:   nil,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertIntegrationCreatedLink(t, link, now)

	stored, err := repository.FindByCode(ctx, "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stored != link {
		t.Fatalf("expected stored %v, got %v", stored, link)
	}
}

func assertIntegrationCreatedLink(t *testing.T, link domain.Link, expectedTime time.Time) {
	t.Helper()

	if link.Code() != "abc123" {
		t.Fatalf("expected code %q, got %q", "abc123", link.Code())
	}

	if link.Destination().String() != "https://example.com/Path?q=TinyURL#Section" {
		t.Fatalf("expected destination %q, got %q", "https://example.com/Path?q=TinyURL#Section", link.Destination().String())
	}

	if link.OwnerID() != "owner-1" {
		t.Fatalf("expected owner ID %q, got %q", "owner-1", link.OwnerID())
	}

	if link.Status() != domain.Active {
		t.Fatalf("expected status %v, got %v", domain.Active, link.Status())
	}

	if !link.CreatedAt().Equal(expectedTime) {
		t.Fatalf("expected createdAt %v, got %v", expectedTime, link.CreatedAt())
	}

	if !link.UpdatedAt().Equal(expectedTime) {
		t.Fatalf("expected updatedAt %v, got %v", expectedTime, link.UpdatedAt())
	}

	if link.ExpiresAt() != nil {
		t.Fatalf("expected nil expiration, got %v", link.ExpiresAt())
	}

	if link.Version() != 1 {
		t.Fatalf("expected version %d, got %d", uint64(1), link.Version())
	}
}
