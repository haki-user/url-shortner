package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type redirectLinkRepositoryFake struct {
	findCalls    int
	receivedCtx  context.Context
	receivedCode string

	link domain.Link
	err  error
}

func (f *redirectLinkRepositoryFake) Insert(ctx context.Context, link domain.Link) error {
	return nil
}

func (f *redirectLinkRepositoryFake) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	f.findCalls++
	f.receivedCtx = ctx
	f.receivedCode = code

	return f.link, f.err
}

func (f *redirectLinkRepositoryFake) Update(ctx context.Context, link domain.Link, expectedVersion uint64) error {
	return nil
}

type redirectLinkClockFake struct {
	nowCalls int
	now      time.Time
}

func (f *redirectLinkClockFake) Now() time.Time {
	f.nowCalls++

	return f.now
}

func TestRedirectLinkActiveUnexpiredLinkRedirects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	link := mustNewRedirectLinkTestLink(t, "abc123", "https://example.com", now, nil)

	repository := &redirectLinkRepositoryFake{link: link}
	clock := &redirectLinkClockFake{now: now}

	useCase := NewRedirectLink(repository, clock)

	result, err := useCase.Execute(ctx, RedirectLinkRequest{
		Code: "abc123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Destination != "https://example.com" {
		t.Fatalf("expected destination %q, got %q", "https://example.com", result.Destination)
	}

	if repository.findCalls != 1 {
		t.Fatalf("expected repository to be called once, got %d", repository.findCalls)
	}

	if repository.receivedCtx != ctx {
		t.Fatal("expected repository to receive original context")
	}

	if repository.receivedCode != "abc123" {
		t.Fatalf("expected repository to receive code %q, got %q", "abc123", repository.receivedCode)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}
}

func TestRedirectLinkMissingLinkReturnsRepositoryErrorUnchanged(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &redirectLinkRepositoryFake{err: ports.ErrLinkNotFound}
	clock := &redirectLinkClockFake{now: now}

	useCase := NewRedirectLink(repository, clock)

	result, err := useCase.Execute(ctx, RedirectLinkRequest{
		Code: "missing",
	})
	if !errors.Is(err, ports.ErrLinkNotFound) {
		t.Fatalf("expected error %v, got %v", ports.ErrLinkNotFound, err)
	}

	if result.Destination != "" {
		t.Fatalf("expected empty destination, got %q", result.Destination)
	}

	if repository.findCalls != 1 {
		t.Fatalf("expected repository to be called once, got %d", repository.findCalls)
	}

	if clock.nowCalls != 0 {
		t.Fatalf("expected clock not to be called, got %d", clock.nowCalls)
	}
}

func TestRedirectLinkUnavailableStatusesAndTimes(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)

	tests := []struct {
		name  string
		setup func(t *testing.T) domain.Link
		now   time.Time
	}{
		{
			name: "disabled link returns unavailable",
			setup: func(t *testing.T) domain.Link {
				link := mustNewRedirectLinkTestLink(t, "abc123", "https://example.com", now, nil)

				err := link.Disable(now.Add(time.Minute))
				if err != nil {
					t.Fatalf("expected disable setup to succeed, got %v", err)
				}

				return link
			},
			now: now.Add(2 * time.Minute),
		},
		{
			name: "deleted link returns unavailable",
			setup: func(t *testing.T) domain.Link {
				link := mustNewRedirectLinkTestLink(t, "abc123", "https://example.com", now, nil)

				err := link.Delete(now.Add(time.Minute))
				if err != nil {
					t.Fatalf("expected delete setup to succeed, got %v", err)
				}

				return link
			},
			now: now.Add(2 * time.Minute),
		},
		{
			name: "expired link returns unavailable",
			setup: func(t *testing.T) domain.Link {
				return mustNewRedirectLinkTestLink(t, "abc123", "https://example.com", now, &expiresAt)
			},
			now: expiresAt,
		},
		{
			name: "zero clock time returns unavailable",
			setup: func(t *testing.T) domain.Link {
				return mustNewRedirectLinkTestLink(t, "abc123", "https://example.com", now, nil)
			},
			now: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			link := tt.setup(t)

			repository := &redirectLinkRepositoryFake{link: link}
			clock := &redirectLinkClockFake{now: tt.now}

			useCase := NewRedirectLink(repository, clock)

			result, err := useCase.Execute(ctx, RedirectLinkRequest{
				Code: "abc123",
			})
			if !errors.Is(err, ErrLinkUnavailable) {
				t.Fatalf("expected error %v, got %v", ErrLinkUnavailable, err)
			}

			if result.Destination != "" {
				t.Fatalf("expected empty destination, got %q", result.Destination)
			}

			if repository.findCalls != 1 {
				t.Fatalf("expected repository to be called once, got %d", repository.findCalls)
			}

			if clock.nowCalls != 1 {
				t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
			}
		})
	}
}

func mustNewRedirectLinkTestLink(
	t *testing.T,
	code string,
	rawDestination string,
	createdAt time.Time,
	expiresAt *time.Time,
) domain.Link {
	t.Helper()

	destination, err := domain.NewDestinationURL(rawDestination)
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	link, err := domain.NewLink(code, destination, "owner-1", createdAt, expiresAt)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	return link
}
