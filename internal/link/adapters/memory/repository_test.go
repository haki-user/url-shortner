package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

func TestNewRepositoryInitializesMap(t *testing.T) {
	repository := NewRepository()

	if repository == nil {
		t.Fatal("expected repository, got nil")
	}

	if repository.links == nil {
		t.Fatal("expected links map to be initialized")
	}
}

func TestRepositoryInsertSucceeds(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	stored, exists := repository.links[link.Code()]
	if !exists {
		t.Fatal("expected link to be stored")
	}

	if stored != link {
		t.Fatalf("expected stored link %v, got %v", link, stored)
	}
}

func TestRepositoryInsertDuplicateReturnsAlreadyExists(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected first insert to succeed, got %v", err)
	}

	err = repository.Insert(context.Background(), link)
	if !errors.Is(err, ports.ErrLinkAlreadyExists) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkAlreadyExists, err)
	}
}

func TestRepositoryInsertDuplicateDoesNotOverwriteOriginal(t *testing.T) {
	repository := NewRepository()

	original := mustNewMemoryRepositoryTestLink(t, "abc123", "https://original.example.com")
	duplicate := mustNewMemoryRepositoryTestLink(t, "abc123", "https://duplicate.example.com")

	err := repository.Insert(context.Background(), original)
	if err != nil {
		t.Fatalf("expected first insert to succeed, got %v", err)
	}

	err = repository.Insert(context.Background(), duplicate)
	if !errors.Is(err, ports.ErrLinkAlreadyExists) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkAlreadyExists, err)
	}

	stored := repository.links["abc123"]
	if stored != original {
		t.Fatalf("expected original link to remain stored, got %v", stored)
	}

	if stored.Destination().String() != "https://original.example.com" {
		t.Fatalf("expected original destination to remain, got %q", stored.Destination().String())
	}
}

func TestRepositoryInsertCancelledContextReturnsContextCanceled(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repository.Insert(ctx, link)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}
}

func TestRepositoryInsertCancelledContextStoresNothing(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repository.Insert(ctx, link)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	if len(repository.links) != 0 {
		t.Fatalf("expected no stored links, got %d", len(repository.links))
	}
}

func TestRepositoryFindByCodeReturnsStoredLink(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	found, err := repository.FindByCode(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if found != link {
		t.Fatalf("expected link %v, got %v", link, found)
	}
}

func TestRepositoryFindByCodeMissingCodeReturnsNotFound(t *testing.T) {
	repository := NewRepository()

	found, err := repository.FindByCode(context.Background(), "missing")
	if !errors.Is(err, ports.ErrLinkNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkNotFound, err)
	}

	if !memoryRepositoryLinkIsZero(found) {
		t.Fatal("expected zero-value link")
	}
}

func TestRepositoryFindByCodeCancelledContextReturnsContextCanceled(t *testing.T) {
	repository := NewRepository()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	found, err := repository.FindByCode(ctx, "abc123")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	if !memoryRepositoryLinkIsZero(found) {
		t.Fatal("expected zero-value link")
	}
}

func TestRepositoryFindByCodeCancelledContextWinsEvenWhenCodeExists(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	found, err := repository.FindByCode(ctx, "abc123")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	if !memoryRepositoryLinkIsZero(found) {
		t.Fatal("expected zero-value link")
	}
}

func TestRepositoryUpdateExistingLinkWithMatchingExpectedVersionSucceeds(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	updated := link
	updatedAt := updated.CreatedAt().Add(time.Hour)

	err = updated.Disable(updatedAt)
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	err = repository.Update(context.Background(), updated, 1)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	stored := repository.links["abc123"]
	if stored != updated {
		t.Fatalf("expected stored link %v, got %v", updated, stored)
	}
}

func TestRepositoryUpdateReplacesStoredLink(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	updated := link
	updatedAt := updated.CreatedAt().Add(time.Hour)

	err = updated.Disable(updatedAt)
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	err = repository.Update(context.Background(), updated, link.Version())
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	stored, err := repository.FindByCode(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("expected find to succeed, got %v", err)
	}

	if stored.Status() != domain.Disabled {
		t.Fatalf("expected stored status %v, got %v", domain.Disabled, stored.Status())
	}

	if stored.Version() != 2 {
		t.Fatalf("expected stored version %d, got %d", uint64(2), stored.Version())
	}

	if !stored.UpdatedAt().Equal(updatedAt) {
		t.Fatalf("expected stored updatedAt %v, got %v", updatedAt, stored.UpdatedAt())
	}
}

func TestRepositoryUpdateMissingLinkReturnsNotFound(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "missing", "https://example.com")

	err := repository.Update(context.Background(), link, 1)
	if !errors.Is(err, ports.ErrLinkNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkNotFound, err)
	}
}

func TestRepositoryUpdateVersionMismatchReturnsVersionConflict(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	updated := link
	err = updated.Disable(updated.CreatedAt().Add(time.Hour))
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	err = repository.Update(context.Background(), updated, 999)
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("expected error %v, got %v", ports.ErrVersionConflict, err)
	}
}

func TestRepositoryUpdateVersionMismatchDoesNotOverwriteStoredLink(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	updated := link
	err = updated.Disable(updated.CreatedAt().Add(time.Hour))
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	err = repository.Update(context.Background(), updated, 999)
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("expected error %v, got %v", ports.ErrVersionConflict, err)
	}

	stored := repository.links["abc123"]
	if stored != link {
		t.Fatalf("expected original stored link to remain %v, got %v", link, stored)
	}

	if stored.Status() != domain.Active {
		t.Fatalf("expected stored status %v, got %v", domain.Active, stored.Status())
	}

	if stored.Version() != 1 {
		t.Fatalf("expected stored version %d, got %d", uint64(1), stored.Version())
	}
}

func TestRepositoryUpdateCancelledContextReturnsContextCanceled(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repository.Update(ctx, link, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}
}

func TestRepositoryUpdateCancelledContextDoesNotOverwriteStoredLink(t *testing.T) {
	repository := NewRepository()
	link := mustNewMemoryRepositoryTestLink(t, "abc123", "https://example.com")

	err := repository.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("expected insert to succeed, got %v", err)
	}

	updated := link
	err = updated.Disable(updated.CreatedAt().Add(time.Hour))
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = repository.Update(ctx, updated, link.Version())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	stored := repository.links["abc123"]
	if stored != link {
		t.Fatalf("expected original stored link to remain %v, got %v", link, stored)
	}

	if stored.Status() != domain.Active {
		t.Fatalf("expected stored status %v, got %v", domain.Active, stored.Status())
	}

	if stored.Version() != 1 {
		t.Fatalf("expected stored version %d, got %d", uint64(1), stored.Version())
	}
}

func TestRepositoryInsertConcurrentUniqueLinks(t *testing.T) {
	repository := NewRepository()

	const totalLinks = 100

	var wg sync.WaitGroup
	errs := make(chan error, totalLinks)

	for i := 0; i < totalLinks; i++ {
		i := i

		wg.Add(1)
		go func() {
			defer wg.Done()

			code := fmt.Sprintf("code-%03d", i)
			destination := fmt.Sprintf("https://example.com/%03d", i)

			link := mustNewMemoryRepositoryTestLink(t, code, destination)

			errs <- repository.Insert(context.Background(), link)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("expected concurrent insert to succeed, got %v", err)
		}
	}

	if len(repository.links) != totalLinks {
		t.Fatalf("expected %d stored links, got %d", totalLinks, len(repository.links))
	}
}

func memoryRepositoryLinkIsZero(link domain.Link) bool {
	return link.Code() == "" &&
		link.Destination().IsZero() &&
		link.OwnerID() == "" &&
		link.Status() == domain.Unknown &&
		link.CreatedAt().IsZero() &&
		link.UpdatedAt().IsZero() &&
		link.ExpiresAt() == nil &&
		link.Version() == 0
}

func mustNewMemoryRepositoryTestLink(t *testing.T, code string, rawDestination string) domain.Link {
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
