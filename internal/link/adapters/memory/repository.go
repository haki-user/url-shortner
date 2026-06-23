package memory

import (
	"context"
	"sync"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

var _ ports.LinkRepository = (*Repository)(nil)

type Repository struct {
	mu    sync.RWMutex
	links map[string]domain.Link
}

func NewRepository() *Repository {
	return &Repository{
		links: make(map[string]domain.Link),
	}
}

func (r *Repository) Insert(ctx context.Context, link domain.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.links[link.Code()]; exists {
		return ports.ErrLinkAlreadyExists
	}

	r.links[link.Code()] = link

	return nil
}

func (r *Repository) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	if err := ctx.Err(); err != nil {
		return domain.Link{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if link, exists := r.links[code]; exists {
		return link, nil
	}

	return domain.Link{}, ports.ErrLinkNotFound
}

func (r *Repository) Update(ctx context.Context, link domain.Link, expectedVersion uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	stored, exists := r.links[link.Code()]
	if !exists {
		return ports.ErrLinkNotFound
	}

	if stored.Version() != expectedVersion {
		return ports.ErrVersionConflict
	}

	r.links[link.Code()] = link

	return nil
}
