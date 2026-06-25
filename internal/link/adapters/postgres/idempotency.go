package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

var _ ports.IdempotencyStore = (*IdempotencyStore)(nil)

type IdempotencyStore struct {
	pool  *pgxpool.Pool
	links *Repository
}

func NewIdempotencyStore(pool *pgxpool.Pool) *IdempotencyStore {
	return &IdempotencyStore{
		pool:  pool,
		links: NewRepository(pool),
	}
}

func (s *IdempotencyStore) Get(ctx context.Context, ownerID string, key string) (domain.Link, error) {
	var code string

	err := s.pool.QueryRow(ctx, `
		select code
		from idempotency_keys
		where owner_id = $1
			and key = $2
	`, ownerID, key).Scan(&code)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, ports.ErrIdempotencyKeyNotFound
		}

		return domain.Link{}, err
	}

	return s.links.FindByCode(ctx, code)
}

func (s *IdempotencyStore) Save(ctx context.Context, ownerID string, key string, link domain.Link) error {
	commandTag, err := s.pool.Exec(ctx, `
		insert into idempotency_keys (owner_id, key, code, created_at)
		values($1, $2, $3, $4)
		on conflict (owner_id, key) do nothing
	`, ownerID, key, link.Code(), link.CreatedAt())
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 1 {
		return nil
	}

	var existingCode string

	err = s.pool.QueryRow(ctx, `
		select code
		from idempotency_keys
		where owner_id = $1
			and key = $2
	`, ownerID, key).Scan(&existingCode)
	if err != nil {
		return err
	}

	if existingCode != link.Code() {
		return ports.ErrIdempotencyKeyConflict
	}

	return nil
}
