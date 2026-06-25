package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
