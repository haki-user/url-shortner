package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"tinyurl/internal/analytics/domain"
)

func TestNewRedirectEventRecorderStartsEmpty(t *testing.T) {
	recorder := NewRedirectEventRecorder()

	if recorder == nil {
		t.Fatal("expected recorder, got nil")
	}

	events := recorder.Events()
	if len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}
}

func TestRedirectEventRecorderRecordStoresEvent(t *testing.T) {
	recorder := NewRedirectEventRecorder()
	event := mustNewRedirectEventRecorderTestEvent(t, "abc123")

	err := recorder.Record(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0] != event {
		t.Fatalf("expected event %v, got %v", event, events[0])
	}
}

func TestRedirectEventRecorderRecordCanceledContextReturnsContextCanceled(t *testing.T) {
	recorder := NewRedirectEventRecorder()
	event := mustNewRedirectEventRecorderTestEvent(t, "abc123")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := recorder.Record(ctx, event)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}
}

func TestRedirectEventRecorderCanceledRecordDoesNotStoreEvent(t *testing.T) {
	recorder := NewRedirectEventRecorder()
	event := mustNewRedirectEventRecorderTestEvent(t, "abc123")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := recorder.Record(ctx, event)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	events := recorder.Events()
	if len(events) != 0 {
		t.Fatalf("expected no stored events, got %d", len(events))
	}
}

func TestRedirectEventRecorderEventsReturnsCopy(t *testing.T) {
	recorder := NewRedirectEventRecorder()
	event := mustNewRedirectEventRecorderTestEvent(t, "abc123")

	err := recorder.Record(context.Background(), event)
	if err != nil {
		t.Fatalf("expected record to succeed, got %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	events[0] = mustNewRedirectEventRecorderTestEvent(t, "changed")

	eventsAgain := recorder.Events()
	if len(eventsAgain) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventsAgain))
	}

	if eventsAgain[0] != event {
		t.Fatalf("expected internal event to remain %v, got %v", event, eventsAgain[0])
	}
}

func TestRedirectEventRecorderConcurrentRecordStoresAllEvents(t *testing.T) {
	recorder := NewRedirectEventRecorder()

	const totalEvents = 100

	var wg sync.WaitGroup
	errs := make(chan error, totalEvents)

	for i := 0; i < totalEvents; i++ {
		i := i

		wg.Add(1)
		go func() {
			defer wg.Done()

			event := mustNewRedirectEventRecorderTestEvent(t, fmt.Sprintf("code-%03d", i))
			errs <- recorder.Record(context.Background(), event)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}

	events := recorder.Events()
	if len(events) != totalEvents {
		t.Fatalf("expected %d events, got %d", totalEvents, len(events))
	}
}

func mustNewRedirectEventRecorderTestEvent(t *testing.T, code string) domain.RedirectEvent {
	t.Helper()

	event, err := domain.NewRedirectEvent(
		code,
		time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC),
		"Mozilla/5.0",
		"https://referer.example.com",
		"127.0.0.1",
	)
	if err != nil {
		t.Fatalf("expected event setup to succeed, got %v", err)
	}

	return event
}
