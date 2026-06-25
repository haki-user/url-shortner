package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"tinyurl/internal/analytics/domain"
	"tinyurl/internal/analytics/ports"
)

var _ ports.RedirectEventRecorder = (*RedirectEventRecorder)(nil)

type RedirectEventRecorder struct {
	pool *pgxpool.Pool
}

func NewRedirectEventRecorder(pool *pgxpool.Pool) *RedirectEventRecorder {
	return &RedirectEventRecorder{
		pool: pool,
	}
}

func (r *RedirectEventRecorder) Record(ctx context.Context, event domain.RedirectEvent) error {
	_, err := r.pool.Exec(ctx, `
		insert into redirect_events (
			code,
			occurred_at,
			user_agent,
			referer,
			ip
		)
		values ($1, $2, $3, $4, $5)
	`,
		event.Code,
		event.OccurredAt,
		event.UserAgent,
		event.Referer,
		event.IP,
	)

	return err
}
