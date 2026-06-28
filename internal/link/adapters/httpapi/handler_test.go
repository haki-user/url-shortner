package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	analyticsdomain "tinyurl/internal/analytics/domain"
	"tinyurl/internal/link/adapters/memory"
	"tinyurl/internal/link/application"
	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type handlerRepositoryFake struct {
	link      domain.Link
	findErr   error
	insertErr error
}

func (f *handlerRepositoryFake) Insert(ctx context.Context, link domain.Link) error {
	f.link = link

	return f.insertErr
}

func (f *handlerRepositoryFake) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	if f.findErr != nil {
		return domain.Link{}, f.findErr
	}

	return f.link, nil
}

func (f *handlerRepositoryFake) Update(ctx context.Context, link domain.Link, expectedVersion uint64) error {
	return nil
}

type handlerCodeGeneratorFake struct {
	code string
	err  error
}

func (f handlerCodeGeneratorFake) Generate(ctx context.Context) (string, error) {
	return f.code, f.err
}

type handlerClockFake struct {
	now time.Time
}

func (f handlerClockFake) Now() time.Time {
	return f.now
}

type handlerIdempotencyStoreFake struct {
	getCalls int
	getOwner string
	getKey   string
	getLink  domain.Link
	getErr   error

	saveCalls int
	saveOwner string
	saveKey   string
	saveLink  domain.Link
	saveErr   error
}

func (f *handlerIdempotencyStoreFake) Get(
	ctx context.Context,
	ownerID string,
	key string,
) (domain.Link, error) {
	f.getCalls++
	f.getOwner = ownerID
	f.getKey = key

	return f.getLink, f.getErr
}

func (f *handlerIdempotencyStoreFake) Save(
	ctx context.Context,
	ownerID string,
	key string,
	link domain.Link,
) error {
	f.saveCalls++
	f.saveOwner = ownerID
	f.saveKey = key
	f.saveLink = link

	return f.saveErr
}

type handlerAnalyticsRecorderFake struct {
	recordCalls int
	event       analyticsdomain.RedirectEvent
	err         error
}

func (f *handlerAnalyticsRecorderFake) Record(
	ctx context.Context,
	event analyticsdomain.RedirectEvent,
) error {
	f.recordCalls++
	f.event = event

	return f.err
}

func TestHandlerPostLinksSuccessReturnsCreatedJSON(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &handlerRepositoryFake{}
	generator := handlerCodeGeneratorFake{code: "abc123"}
	clock := handlerClockFake{now: now}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	body := bytes.NewBufferString(`{"destination":"https://example.com","ownerId":"owner-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/links", body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	var decoded struct {
		Code        string `json:"code"`
		ShortURL    string `json:"shortUrl"`
		Destination string `json:"destination"`
	}

	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("expected valid JSON response, got %v", err)
	}

	if decoded.Code != "abc123" {
		t.Fatalf("expected code %q, got %q", "abc123", decoded.Code)
	}

	if decoded.ShortURL != "http://localhost:8080/abc123" {
		t.Fatalf("expected shortUrl %q, got %q", "http://localhost:8080/abc123", decoded.ShortURL)
	}

	if decoded.Destination != "https://example.com" {
		t.Fatalf("expected destination %q, got %q", "https://example.com", decoded.Destination)
	}
}

func TestHandlerPostLinksInvalidJSONReturnsBadRequest(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &handlerRepositoryFake{}
	generator := handlerCodeGeneratorFake{code: "abc123"}
	clock := handlerClockFake{now: now}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	request := httptest.NewRequest(http.MethodPost, "/v1/links", bytes.NewBufferString(`{bad json`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestHandlerGetCodeRedirectsWithLocation(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	link := mustNewHandlerTestLink(t, "abc123", "https://example.com", now, nil)

	repository := &handlerRepositoryFake{link: link}
	generator := handlerCodeGeneratorFake{code: "unused"}
	clock := handlerClockFake{now: now}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	request := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}

	if response.Header().Get("Location") != "https://example.com" {
		t.Fatalf("expected Location %q, got %q", "https://example.com", response.Header().Get("Location"))
	}
}

func TestHandlerGetMissingCodeReturnsNotFound(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &handlerRepositoryFake{findErr: ports.ErrLinkNotFound}
	generator := handlerCodeGeneratorFake{code: "unused"}
	clock := handlerClockFake{now: now}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestHandlerPostLinksWithIdempotencyKeyPassesKeyToCreateUseCase(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &handlerRepositoryFake{}
	generator := handlerCodeGeneratorFake{code: "abc123"}
	clock := handlerClockFake{now: now}
	idempotencyStore := &handlerIdempotencyStoreFake{
		getErr: ports.ErrIdempotencyKeyNotFound,
	}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock, idempotencyStore),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	body := bytes.NewBufferString(`{"destination":"https://example.com","ownerId":"owner-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/links", body)
	request.Header.Set("Idempotency-Key", "retry-key-1")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	if idempotencyStore.getCalls != 1 {
		t.Fatalf("expected idempotency get to be called once, got %d", idempotencyStore.getCalls)
	}

	if idempotencyStore.getOwner != "owner-1" {
		t.Fatalf("expected get owner %q, got %q", "owner-1", idempotencyStore.getOwner)
	}

	if idempotencyStore.getKey != "retry-key-1" {
		t.Fatalf("expected get key %q, got %q", "retry-key-1", idempotencyStore.getKey)
	}

	if idempotencyStore.saveCalls != 1 {
		t.Fatalf("expected idempotency save to be called once, got %d", idempotencyStore.saveCalls)
	}

	if idempotencyStore.saveOwner != "owner-1" {
		t.Fatalf("expected save owner %q, got %q", "owner-1", idempotencyStore.saveOwner)
	}

	if idempotencyStore.saveKey != "retry-key-1" {
		t.Fatalf("expected save key %q, got %q", "retry-key-1", idempotencyStore.saveKey)
	}
}

func TestHandlerPostLinksWithoutIdempotencyKeySkipsIdempotency(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := &handlerRepositoryFake{}
	generator := handlerCodeGeneratorFake{code: "abc123"}
	clock := handlerClockFake{now: now}
	idempotencyStore := &handlerIdempotencyStoreFake{
		getErr: ports.ErrIdempotencyKeyNotFound,
	}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock, idempotencyStore),
		application.NewRedirectLink(application.NewRepositoryResolver(repository), clock),
		"http://localhost:8080",
	)

	body := bytes.NewBufferString(`{"destination":"https://example.com","ownerId":"owner-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/links", body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	if idempotencyStore.getCalls != 0 {
		t.Fatalf("expected idempotency get not to be called, got %d", idempotencyStore.getCalls)
	}

	if idempotencyStore.saveCalls != 0 {
		t.Fatalf("expected idempotency save not to be called, got %d", idempotencyStore.saveCalls)
	}
}

func TestHandlerPostLinksSameIdempotencyKeyReturnsSameCode(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)

	repository := memory.NewRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	generator := handlerCodeGeneratorFake{code: "abc123"}
	clock := handlerClockFake{now: now}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock, idempotencyStore),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/links",
		bytes.NewBufferString(`{"destination":"https://example.com","ownerId":"owner-1"}`),
	)
	firstRequest.Header.Set("Idempotency-Key", "retry-key-1")
	firstResponse := httptest.NewRecorder()

	handler.ServeHTTP(firstResponse, firstRequest)

	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("expected first status %d, got %d", http.StatusCreated, firstResponse.Code)
	}

	var firstDecoded struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(firstResponse.Body).Decode(&firstDecoded); err != nil {
		t.Fatalf("expected first response JSON, got %v", err)
	}

	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/links",
		bytes.NewBufferString(`{"destination":"https://example.com","ownerId":"owner-1"}`),
	)
	secondRequest.Header.Set("Idempotency-Key", "retry-key-1")
	secondResponse := httptest.NewRecorder()

	handler.ServeHTTP(secondResponse, secondRequest)

	if secondResponse.Code != http.StatusCreated {
		t.Fatalf("expected second status %d, got %d", http.StatusCreated, secondResponse.Code)
	}

	var secondDecoded struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(secondResponse.Body).Decode(&secondDecoded); err != nil {
		t.Fatalf("expected second response JSON, got %v", err)
	}

	if secondDecoded.Code != firstDecoded.Code {
		t.Fatalf("expected same code %q, got %q", firstDecoded.Code, secondDecoded.Code)
	}
}

func TestHandlerGetCodeRecordsAnalyticsEvent(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	link := mustNewHandlerTestLink(t, "abc123", "https://example.com", now, nil)

	repository := &handlerRepositoryFake{link: link}
	generator := handlerCodeGeneratorFake{code: "unused"}
	clock := handlerClockFake{now: now}
	analyticsRecorder := &handlerAnalyticsRecorderFake{}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
		WithAnalytics(analyticsRecorder, clock),
	)

	request := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Referer", "https://referer.example.com")
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}

	if response.Header().Get("Location") != "https://example.com" {
		t.Fatalf("expected Location %q, got %q", "https://example.com", response.Header().Get("Location"))
	}

	if analyticsRecorder.recordCalls != 1 {
		t.Fatalf("expected analytics recorder to be called once, got %d", analyticsRecorder.recordCalls)
	}

	event := analyticsRecorder.event

	if event.Code != "abc123" {
		t.Fatalf("expected event code %q, got %q", "abc123", event.Code)
	}

	if !event.OccurredAt.Equal(now) {
		t.Fatalf("expected occurredAt %v, got %v", now, event.OccurredAt)
	}

	if event.UserAgent != "Mozilla/5.0" {
		t.Fatalf("expected user agent %q, got %q", "Mozilla/5.0", event.UserAgent)
	}

	if event.Referer != "https://referer.example.com" {
		t.Fatalf("expected referer %q, got %q", "https://referer.example.com", event.Referer)
	}

	if event.IP != "203.0.113.10" {
		t.Fatalf("expected IP %q, got %q", "203.0.113.10", event.IP)
	}
}

func TestHandlerGetCodeStillRedirectsWhenAnalyticsRecorderFails(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	link := mustNewHandlerTestLink(t, "abc123", "https://example.com", now, nil)

	repository := &handlerRepositoryFake{link: link}
	generator := handlerCodeGeneratorFake{code: "unused"}
	clock := handlerClockFake{now: now}
	analyticsRecorder := &handlerAnalyticsRecorderFake{
		err: errors.New("analytics failed"),
	}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
		WithAnalytics(analyticsRecorder, clock),
	)

	request := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}

	if response.Header().Get("Location") != "https://example.com" {
		t.Fatalf("expected Location %q, got %q", "https://example.com", response.Header().Get("Location"))
	}

	if analyticsRecorder.recordCalls != 1 {
		t.Fatalf("expected analytics recorder to be called once, got %d", analyticsRecorder.recordCalls)
	}
}

func TestHandlerGetCodeWithoutAnalyticsRecorderStillRedirects(t *testing.T) {
	now := time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)
	link := mustNewHandlerTestLink(t, "abc123", "https://example.com", now, nil)

	repository := &handlerRepositoryFake{link: link}
	generator := handlerCodeGeneratorFake{code: "unused"}
	clock := handlerClockFake{now: now}

	handler := NewHandler(
		application.NewCreateGeneratedLink(repository, generator, clock),
		application.NewRedirectLink(
			application.NewRepositoryResolver(repository),
			clock,
		),
		"http://localhost:8080",
	)

	request := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}

	if response.Header().Get("Location") != "https://example.com" {
		t.Fatalf("expected Location %q, got %q", "https://example.com", response.Header().Get("Location"))
	}
}

func mustNewHandlerTestLink(
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
