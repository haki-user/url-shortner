package ports

import (
	"context"

	"tinyurl/internal/link/domain"
)

// LinkResolver resolves the redirect-facing projection for a short code.
// Implementations may use the repository directly or a cache-aside strategy.
type LinkResolver interface {
	Resolve(ctx context.Context, code string) (domain.RedirectMapping, error)
}
