package ports

import (
	"context"

	"tinyurl/internal/analytics/domain"
)

type RedirectEventRecorder interface {
	Record(ctx context.Context, event domain.RedirectEvent) error
}
