package ports

import (
	"context"
	"errors"
	"tinyurl/internal/link/domain"
)

var (
	ErrLinkNotFound      = errors.New("link not found")
	ErrLinkAlreadyExists = errors.New("link already exists")
	ErrVersionConflict   = errors.New("version conflict")
)

type LinkRepository interface {
	Insert(ctx context.Context, link domain.Link) error
	FindByCode(ctx context.Context, code string) (domain.Link, error)
	Update(ctx context.Context, link domain.Link, expectedVersion uint64) error
}
