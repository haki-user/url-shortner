package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type createGeneratedLinkRepositoryFake struct {
	insertCalls int
	insertCtx   context.Context
	inserted    domain.Link
	insertErr   error
}

func (f *createGeneratedLinkRepositoryFake) Insert(ctx context.Context, link domain.Link) error {
	f.insertCalls++
	f.insertCtx = ctx
	f.inserted = link

	return f.insertErr
}

func (f *createGeneratedLinkRepositoryFake) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	return domain.Link{}, nil
}

func (f *createGeneratedLinkRepositoryFake) Update(ctx context.Context, link domain.Link, expectedVersion uint64) error {
	return nil
}

type createGeneratedLinkGeneratorFake struct {
	generateCalls int
	generateCtx   context.Context
	code          string
	err           error
}

func (f *createGeneratedLinkGeneratorFake) Generate(ctx context.Context) (string, error) {
	f.generateCalls++
	f.generateCtx = ctx

	return f.code, f.err
}

type createGeneratedLinkClockFake struct {
	nowCalls int
	now      time.Time
}

func (f *createGeneratedLinkClockFake) Now() time.Time {
	f.nowCalls++

	return f.now
}

type createGeneratedLinkIdempotencyStoreFake struct {
	getCalls int
	getCtx   context.Context
	getOwner string
	getKey   string
	getLink  domain.Link
	getErr   error

	saveCalls int
	saveCtx   context.Context
	saveOwner string
	saveKey   string
	saveLink  domain.Link
	saveErr   error
}

func (f *createGeneratedLinkIdempotencyStoreFake) Get(
	ctx context.Context,
	ownerID string,
	key string,
) (domain.Link, error) {
	f.getCalls++
	f.getCtx = ctx
	f.getOwner = ownerID
	f.getKey = key

	return f.getLink, f.getErr
}

func (f *createGeneratedLinkIdempotencyStoreFake) Save(
	ctx context.Context,
	ownerID string,
	key string,
	link domain.Link,
) error {
	f.saveCalls++
	f.saveCtx = ctx
	f.saveOwner = ownerID
	f.saveKey = key
	f.saveLink = link

	return f.saveErr
}

func TestCreateGeneratedLinkSuccessReturnsAndInsertsCorrectLink(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Date(2026, 6, 14, 13, 15, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "HTTPS://EXAMPLE.COM/Path?q=TinyURL#Section",
		OwnerID:     "owner-1",
		ExpiresAt:   &expiresAt,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if generator.generateCtx != ctx {
		t.Fatal("expected generator to receive original context")
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 1 {
		t.Fatalf("expected repository insert to be called once, got %d", repository.insertCalls)
	}

	if repository.insertCtx != ctx {
		t.Fatal("expected repository to receive original context")
	}

	assertCreatedLink(t, link, "abc123", "https://example.com/Path?q=TinyURL#Section", "owner-1", now, &expiresAt)
	assertCreatedLink(t, repository.inserted, "abc123", "https://example.com/Path?q=TinyURL#Section", "owner-1", now, &expiresAt)
}

func TestCreateGeneratedLinkInvalidDestinationStopsBeforeGeneratorClockAndRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "://bad-url",
		OwnerID:     "owner-1",
		ExpiresAt:   nil,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, domain.ErrMalformedDestination) {
		t.Fatalf("expected error %v, got %v", domain.ErrMalformedDestination, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 0 {
		t.Fatalf("expected generator not to be called, got %d calls", generator.generateCalls)
	}

	if clock.nowCalls != 0 {
		t.Fatalf("expected clock not to be called, got %d calls", clock.nowCalls)
	}

	if repository.insertCalls != 0 {
		t.Fatalf("expected repository not to be called, got %d calls", repository.insertCalls)
	}
}

func TestCreateGeneratedLinkGeneratorFailureStopsBeforeClockAndRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	expectedErr := errors.New("generate failed")

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{err: expectedErr}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "https://example.com",
		OwnerID:     "owner-1",
		ExpiresAt:   nil,
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if generator.generateCtx != ctx {
		t.Fatal("expected generator to receive original context")
	}

	if clock.nowCalls != 0 {
		t.Fatalf("expected clock not to be called, got %d calls", clock.nowCalls)
	}

	if repository.insertCalls != 0 {
		t.Fatalf("expected repository not to be called, got %d calls", repository.insertCalls)
	}
}

func TestCreateGeneratedLinkInvalidGeneratedCodeSkipsRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "   "}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "https://example.com",
		OwnerID:     "owner-1",
		ExpiresAt:   nil,
	})
	if !errors.Is(err, domain.ErrEmptyCode) {
		t.Fatalf("expected error %v, got %v", domain.ErrEmptyCode, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 0 {
		t.Fatalf("expected repository not to be called, got %d calls", repository.insertCalls)
	}
}

func TestCreateGeneratedLinkInvalidExpirationStopsBeforeRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	invalidExpiresAt := now.Add(-time.Hour) // expiration <= now

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "https://example.com",
		OwnerID:     "owner-1",
		ExpiresAt:   &invalidExpiresAt,
	})
	if !errors.Is(err, domain.ErrInvalidExpiresAt) {
		t.Fatalf("expected error %v, got %v", domain.ErrInvalidExpiresAt, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 0 {
		t.Fatalf("expected repository not to be called, got %d calls", repository.insertCalls)
	}
}

func TestCreateGeneratedLinkRepositoryFailureIsReturnedUnchanged(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	expectedErr := ports.ErrLinkAlreadyExists

	repository := &createGeneratedLinkRepositoryFake{insertErr: expectedErr}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "https://example.com",
		OwnerID:     "owner-1",
		ExpiresAt:   nil,
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 1 {
		t.Fatalf("expected repository insert to be called once, got %d", repository.insertCalls)
	}

	if repository.insertCtx != ctx {
		t.Fatal("expected repository to receive original context")
	}
}

func TestCreateGeneratedLinkPassesExpirationIntoCreatedLink(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	expiresAt := now.Add(7 * 24 * time.Hour)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}

	useCase := NewCreateGeneratedLink(repository, generator, clock)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "https://example.com",
		OwnerID:     "owner-1",
		ExpiresAt:   &expiresAt,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatal("expected expiration, got nil")
	}

	if !actualExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expiration %v, got %v", expiresAt, *actualExpiresAt)
	}

	insertedExpiresAt := repository.inserted.ExpiresAt()
	if insertedExpiresAt == nil {
		t.Fatal("expected inserted expiration, got nil")
	}

	if !insertedExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected inserted expiration %v, got %v", expiresAt, *insertedExpiresAt)
	}
}

func TestCreateGeneratedLinkNoIdempotencyKeyKeepsOldBehavior(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}
	idempotencyStore := &createGeneratedLinkIdempotencyStoreFake{}

	useCase := NewCreateGeneratedLink(repository, generator, clock, idempotencyStore)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination: "https://example.com",
		OwnerID:     "owner-1",
		ExpiresAt:   nil,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertCreatedLink(t, link, "abc123", "https://example.com", "owner-1", now, nil)

	if idempotencyStore.getCalls != 0 {
		t.Fatalf("expected idempotency get not to be called, got %d", idempotencyStore.getCalls)
	}

	if idempotencyStore.saveCalls != 0 {
		t.Fatalf("expected idempotency save not to be called, got %d", idempotencyStore.saveCalls)
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 1 {
		t.Fatalf("expected repository insert to be called once, got %d", repository.insertCalls)
	}
}

func TestCreateGeneratedLinkExistingIdempotencyKeyReturnsStoredLink(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	storedLink := mustNewCreateGeneratedLinkTestLink(t, "saved123", "https://saved.example.com", "owner-1", now, nil)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}
	idempotencyStore := &createGeneratedLinkIdempotencyStoreFake{
		getLink: storedLink,
		getErr:  nil,
	}

	useCase := NewCreateGeneratedLink(repository, generator, clock, idempotencyStore)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination:    "https://example.com",
		OwnerID:        "owner-1",
		ExpiresAt:      nil,
		IdempotencyKey: "retry-key-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link != storedLink {
		t.Fatalf("expected stored link %v, got %v", storedLink, link)
	}

	if idempotencyStore.getCalls != 1 {
		t.Fatalf("expected idempotency get to be called once, got %d", idempotencyStore.getCalls)
	}

	if idempotencyStore.getCtx != ctx {
		t.Fatal("expected idempotency get to receive original context")
	}

	if idempotencyStore.getOwner != "owner-1" {
		t.Fatalf("expected owner %q, got %q", "owner-1", idempotencyStore.getOwner)
	}

	if idempotencyStore.getKey != "retry-key-1" {
		t.Fatalf("expected key %q, got %q", "retry-key-1", idempotencyStore.getKey)
	}

	if generator.generateCalls != 0 {
		t.Fatalf("expected generator not to be called, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 0 {
		t.Fatalf("expected clock not to be called, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 0 {
		t.Fatalf("expected repository not to be called, got %d", repository.insertCalls)
	}

	if idempotencyStore.saveCalls != 0 {
		t.Fatalf("expected idempotency save not to be called, got %d", idempotencyStore.saveCalls)
	}
}

func TestCreateGeneratedLinkMissingIdempotencyKeyCreatesAndSavesLink(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}
	idempotencyStore := &createGeneratedLinkIdempotencyStoreFake{
		getErr: ports.ErrIdempotencyKeyNotFound,
	}

	useCase := NewCreateGeneratedLink(repository, generator, clock, idempotencyStore)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination:    "https://example.com",
		OwnerID:        "owner-1",
		ExpiresAt:      nil,
		IdempotencyKey: "retry-key-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertCreatedLink(t, link, "abc123", "https://example.com", "owner-1", now, nil)

	if idempotencyStore.getCalls != 1 {
		t.Fatalf("expected idempotency get to be called once, got %d", idempotencyStore.getCalls)
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 1 {
		t.Fatalf("expected repository insert to be called once, got %d", repository.insertCalls)
	}

	if idempotencyStore.saveCalls != 1 {
		t.Fatalf("expected idempotency save to be called once, got %d", idempotencyStore.saveCalls)
	}

	if idempotencyStore.saveCtx != ctx {
		t.Fatal("expected idempotency save to receive original context")
	}

	if idempotencyStore.saveOwner != "owner-1" {
		t.Fatalf("expected save owner %q, got %q", "owner-1", idempotencyStore.saveOwner)
	}

	if idempotencyStore.saveKey != "retry-key-1" {
		t.Fatalf("expected save key %q, got %q", "retry-key-1", idempotencyStore.saveKey)
	}

	if idempotencyStore.saveLink != link {
		t.Fatalf("expected saved link %v, got %v", link, idempotencyStore.saveLink)
	}
}

func TestCreateGeneratedLinkIdempotencyGetFailureReturnsError(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	expectedErr := errors.New("idempotency get failed")

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}
	idempotencyStore := &createGeneratedLinkIdempotencyStoreFake{
		getErr: expectedErr,
	}

	useCase := NewCreateGeneratedLink(repository, generator, clock, idempotencyStore)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination:    "https://example.com",
		OwnerID:        "owner-1",
		ExpiresAt:      nil,
		IdempotencyKey: "retry-key-1",
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 0 {
		t.Fatalf("expected generator not to be called, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 0 {
		t.Fatalf("expected clock not to be called, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 0 {
		t.Fatalf("expected repository not to be called, got %d", repository.insertCalls)
	}

	if idempotencyStore.saveCalls != 0 {
		t.Fatalf("expected idempotency save not to be called, got %d", idempotencyStore.saveCalls)
	}
}

func TestCreateGeneratedLinkIdempotencySaveFailureReturnsErrorAfterInsert(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	expectedErr := errors.New("idempotency save failed")

	repository := &createGeneratedLinkRepositoryFake{}
	generator := &createGeneratedLinkGeneratorFake{code: "abc123"}
	clock := &createGeneratedLinkClockFake{now: now}
	idempotencyStore := &createGeneratedLinkIdempotencyStoreFake{
		getErr:  ports.ErrIdempotencyKeyNotFound,
		saveErr: expectedErr,
	}

	useCase := NewCreateGeneratedLink(repository, generator, clock, idempotencyStore)

	link, err := useCase.Execute(ctx, CreateGeneratedLinkRequest{
		Destination:    "https://example.com",
		OwnerID:        "owner-1",
		ExpiresAt:      nil,
		IdempotencyKey: "retry-key-1",
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if !linkIsZero(link) {
		t.Fatal("expected zero-value link")
	}

	if generator.generateCalls != 1 {
		t.Fatalf("expected generator to be called once, got %d", generator.generateCalls)
	}

	if clock.nowCalls != 1 {
		t.Fatalf("expected clock to be called once, got %d", clock.nowCalls)
	}

	if repository.insertCalls != 1 {
		t.Fatalf("expected repository insert to be called once, got %d", repository.insertCalls)
	}

	if idempotencyStore.saveCalls != 1 {
		t.Fatalf("expected idempotency save to be called once, got %d", idempotencyStore.saveCalls)
	}

	if repository.inserted.Code() != "abc123" {
		t.Fatalf("expected inserted link code %q, got %q", "abc123", repository.inserted.Code())
	}
}

func mustNewCreateGeneratedLinkTestLink(
	t *testing.T,
	code string,
	rawDestination string,
	ownerID string,
	createdAt time.Time,
	expiresAt *time.Time,
) domain.Link {
	t.Helper()

	destination, err := domain.NewDestinationURL(rawDestination)
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	link, err := domain.NewLink(code, destination, ownerID, createdAt, expiresAt)
	if err != nil {
		t.Fatalf("expected link setup to succeed, got %v", err)
	}

	return link
}

func assertCreatedLink(
	t *testing.T,
	link domain.Link,
	expectedCode string,
	expectedDestination string,
	expectedOwnerID string,
	expectedCreatedAt time.Time,
	expectedExpiresAt *time.Time,
) {
	t.Helper()

	if link.Code() != expectedCode {
		t.Fatalf("expected code %q, got %q", expectedCode, link.Code())
	}

	if link.Destination().String() != expectedDestination {
		t.Fatalf("expected destination %q, got %q", expectedDestination, link.Destination())
	}

	if link.OwnerID() != expectedOwnerID {
		t.Fatalf("expected ownerID %q, got %q", expectedOwnerID, link.OwnerID())
	}

	if !link.CreatedAt().Equal(expectedCreatedAt) {
		t.Fatalf("expected createdAt %v, got %v", expectedCreatedAt, link.CreatedAt())
	}

	if !link.UpdatedAt().Equal(expectedCreatedAt) {
		t.Fatalf("expected updatedAt %v, got %v", expectedCreatedAt, link.UpdatedAt())
	}

	if link.Version() != 1 {
		t.Fatalf("expected version %d, got %v", 1, link.Version())
	}

	actualExpiresAt := link.ExpiresAt()
	if expectedExpiresAt == nil {
		if actualExpiresAt != nil {
			t.Fatalf("expected nil expiration, got %v", actualExpiresAt)
		}

		return
	}

	if actualExpiresAt == nil {
		t.Fatal("expected expiration, got nil")
	}

	if !actualExpiresAt.Equal(*expectedExpiresAt) {
		t.Fatalf("expected expiration %v, got %v", *expectedExpiresAt, *actualExpiresAt)
	}
}

func linkIsZero(link domain.Link) bool {
	return link.Code() == "" &&
		link.Destination().IsZero() &&
		link.OwnerID() == "" &&
		link.Status() == domain.Unknown &&
		link.CreatedAt().IsZero() &&
		link.UpdatedAt().IsZero() &&
		link.ExpiresAt() == nil &&
		link.Version() == 0
}
