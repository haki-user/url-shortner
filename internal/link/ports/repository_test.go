package ports

import (
	"context"
	"errors"
	"testing"
	"tinyurl/internal/link/domain"
)

var _ LinkRepository = (*fakeRepository)(nil) // need to understand it more later

type fakeRepository struct{}

func (f *fakeRepository) Insert(ctx context.Context, link domain.Link) error {
	return nil
}

func (f *fakeRepository) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	return domain.Link{}, nil
}

func (f *fakeRepository) Update(ctx context.Context, current domain.Link, expectedVersion uint64) error {
	return nil
}

func TestRepositoryErrorsAreSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "link not found", err: ErrLinkNotFound},
		{name: "link already exists", err: ErrLinkAlreadyExists},
		{name: "version conflict", err: ErrVersionConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.err) {
				t.Fatalf("expected errors.Is to identify %v", tt.err)
			}
		})
	}
}
