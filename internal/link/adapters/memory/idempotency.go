package memory

import (
	"context"
	"sync"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

var _ ports.IdempotencyStore = (*IdempotencyStore)(nil)

type IdempotencyStore struct {
	mu      sync.RWMutex
	records map[idempotencyKey]domain.Link
}

type idempotencyKey struct {
	ownerID string
	key     string
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{
		records: make(map[idempotencyKey]domain.Link),
	}
}

func (s *IdempotencyStore) Get(ctx context.Context, ownerID string, key string) (domain.Link, error) {
	if err := ctx.Err(); err != nil {
		return domain.Link{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	link, exists := s.records[idempotencyKey{
		ownerID: ownerID,
		key:     key,
	}]
	if !exists {
		return domain.Link{}, ports.ErrIdempotencyKeyNotFound
	}

	return link, nil
}

func (s *IdempotencyStore) Save(ctx context.Context, ownerID string, key string, link domain.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	recordKey := idempotencyKey{
		ownerID: ownerID,
		key:     key,
	}

	existing, exists := s.records[recordKey]
	if !exists {
		s.records[recordKey] = link
		return nil
	}

	if existing != link {
		return ports.ErrIdempotencyKeyConflict
	}

	return nil
}
