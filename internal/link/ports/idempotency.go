package ports

import (
	"context"
	"errors"

	"tinyurl/internal/link/domain"
)

var (
	ErrIdempotencyKeyNotFound = errors.New("idempotency key not found")
	ErrIdempotencyKeyConflict = errors.New("idempotency key conflict")
)

type IdempotencyStore interface {
	Get(ctx context.Context, ownerID string, key string) (domain.Link, error)
	Save(ctx context.Context, ownerID string, key string, link domain.Link) error
}
