package ports

import (
	"context"
	"errors"
	"time"

	"tinyurl/internal/link/domain"
)

// ErrRedirectCacheMiss means the requested mapping is not cached.
// It is expected control flow, not a dependency failure.
var ErrRedirectCacheMiss = errors.New("redirect cache miss")

// RedirectCache defines the cache operations required by redirect resolution.
// RedirectCache stores temporary copies of redirect mappings.
// Losing cache data must not lose links; Postgres remains the source of truth.
type RedirectCache interface {
	Get(
		ctx context.Context,
		code string,
	) (domain.RedirectMapping, error)

	PutIfNewer(
		ctx context.Context,
		mapping domain.RedirectMapping,
		ttl time.Duration,
	) error
}
