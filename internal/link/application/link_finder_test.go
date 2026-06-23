package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
)

type fakeLinkRepository struct {
	receivedCtx  context.Context
	receivedCode string

	link domain.Link
	err  error
}

func (f *fakeLinkRepository) Insert(ctx context.Context, link domain.Link) error {
	return nil
}

func (f *fakeLinkRepository) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	f.receivedCtx = ctx
	f.receivedCode = code

	return f.link, f.err
}

func (f *fakeLinkRepository) Update(ctx context.Context, link domain.Link, expectedVersion uint64) error {
	return nil
}

func TestLinkFinderFindForwardsInputsAndReturnsRepositoryOutputs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	code := "abc123"

	destination, err := domain.NewDestinationURL("https://example.com")
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	createdAt := time.Date(2026, 6, 14, 13, 15, 0, 0, time.UTC)

	expectedLink, err := domain.NewLink(code, destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	expectedErr := errors.New("repository error")

	repository := &fakeLinkRepository{
		link: expectedLink,
		err:  expectedErr,
	}

	finder := NewLinkFinder(repository)

	actualLink, actualErr := finder.Find(ctx, code)

	if repository.receivedCtx != ctx {
		t.Fatal("expected repository to receive original context")
	}

	if repository.receivedCode != code {
		t.Fatalf("expected repository to receive code %q, got %q", code, repository.receivedCode)
	}

	if actualLink != expectedLink {
		t.Fatalf("expected returned link %v, got %v", expectedLink, actualLink)
	}

	if !errors.Is(actualErr, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, actualErr)
	}
}
