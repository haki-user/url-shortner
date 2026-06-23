package memory

import (
	"context"
	"sync"

	"tinyurl/internal/analytics/domain"
	"tinyurl/internal/analytics/ports"
)

var _ ports.RedirectEventRecorder = (*RedirectEventRecorder)(nil)

type RedirectEventRecorder struct {
	mu     sync.Mutex
	events []domain.RedirectEvent
}

func NewRedirectEventRecorder() *RedirectEventRecorder {
	return &RedirectEventRecorder{
		events: make([]domain.RedirectEvent, 0),
	}
}

func (r *RedirectEventRecorder) Record(ctx context.Context, event domain.RedirectEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)

	return nil
}

func (r *RedirectEventRecorder) Events() []domain.RedirectEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	events := make([]domain.RedirectEvent, len(r.events))
	copy(events, r.events)

	return events
}
