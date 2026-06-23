package ports

import (
	"context"
	"testing"

	"tinyurl/internal/analytics/domain"
)

var _ RedirectEventRecorder = (*fakeRedirectEventRecorder)(nil)

type fakeRedirectEventRecorder struct {
	err error
}

func (f *fakeRedirectEventRecorder) Record(ctx context.Context, event domain.RedirectEvent) error {
	return f.err
}

func TestFakeRedirectEventRecorderImplementsRedirectEventRecorder(t *testing.T) {
	recorder := &fakeRedirectEventRecorder{}

	err := recorder.Record(context.Background(), domain.RedirectEvent{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
