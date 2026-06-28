package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
)

type refreshingRepositorySourceFake struct {
	insertCalls int
	findCalls   int
	updateCalls int
	updateErr   error
}

func (f *refreshingRepositorySourceFake) Insert(
	ctx context.Context,
	link domain.Link,
) error {
	f.insertCalls++
	return nil
}

func (f *refreshingRepositorySourceFake) FindByCode(
	ctx context.Context,
	code string,
) (domain.Link, error) {
	f.findCalls++
	return domain.Link{}, nil
}

func (f *refreshingRepositorySourceFake) Update(
	ctx context.Context,
	link domain.Link,
	expectedVersion uint64,
) error {
	f.updateCalls++
	return f.updateErr
}

type linkMutationRefresherFake struct {
	calls int
	err   error
}

func (f *linkMutationRefresherFake) Refresh(
	ctx context.Context,
	link domain.Link,
) error {
	f.calls++
	return f.err
}

func TestCacheRefreshingRepositoryRefreshesAfterSuccessfulUpdate(t *testing.T) {
	source := &refreshingRepositorySourceFake{}
	refresher := &linkMutationRefresherFake{}
	repository := NewCacheRefreshingRepository(source, refresher)
	link := mustRefreshingRepositoryLink(t)

	err := repository.Update(context.Background(), link, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if source.updateCalls != 1 {
		t.Fatalf("expected one source update, got %d", source.updateCalls)
	}

	if refresher.calls != 1 {
		t.Fatalf("expected one cache refresh, got %d", refresher.calls)
	}
}

func TestCacheRefreshingRepositoryDoesNotRefreshAfterSourceFailure(t *testing.T) {
	sourceErr := errors.New("postgres update failed")
	source := &refreshingRepositorySourceFake{updateErr: sourceErr}
	refresher := &linkMutationRefresherFake{}
	repository := NewCacheRefreshingRepository(source, refresher)

	err := repository.Update(
		context.Background(),
		mustRefreshingRepositoryLink(t),
		1,
	)
	if !errors.Is(err, sourceErr) {
		t.Fatalf("expected source error %v, got %v", sourceErr, err)
	}

	if refresher.calls != 0 {
		t.Fatalf("expected no cache refresh, got %d", refresher.calls)
	}
}

func TestCacheRefreshingRepositoryIgnoresRefreshFailure(t *testing.T) {
	source := &refreshingRepositorySourceFake{}
	refresher := &linkMutationRefresherFake{
		err: errors.New("redis unavailable"),
	}
	repository := NewCacheRefreshingRepository(source, refresher)

	err := repository.Update(
		context.Background(),
		mustRefreshingRepositoryLink(t),
		1,
	)
	if err != nil {
		t.Fatalf("expected committed source update to remain successful, got %v", err)
	}
}

func mustRefreshingRepositoryLink(t *testing.T) domain.Link {
	t.Helper()

	destination, err := domain.NewDestinationURL("https://example.com")
	if err != nil {
		t.Fatalf("create destination: %v", err)
	}

	link, err := domain.NewLink(
		"abc123",
		destination,
		"owner-1",
		time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
		nil,
	)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	return link
}
