package ports

import (
	"context"
	"errors"
	"testing"

	"tinyurl/internal/link/domain"
)

var _ IdempotencyStore = (*fakeIdempotencyStore)(nil)

type fakeIdempotencyStore struct {
	link domain.Link
	err  error
}

func (f *fakeIdempotencyStore) Get(ctx context.Context, ownerID string, key string) (domain.Link, error) {
	return f.link, f.err
}

func (f *fakeIdempotencyStore) Save(ctx context.Context, ownerID string, key string, link domain.Link) error {
	return f.err
}

func TestIdempotencyErrorsAreSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "key not found", err: ErrIdempotencyKeyNotFound},
		{name: "key conflict", err: ErrIdempotencyKeyConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.err) {
				t.Fatalf("expected errors.Is to identify %v", tt.err)
			}
		})
	}
}
