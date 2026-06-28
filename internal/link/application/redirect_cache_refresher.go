package application

import (
	"context"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

// RedirectCacheRefresher updates the redirect cache after an authoritative
// link mutation has committed.
type RedirectCacheRefresher struct {
	cache            ports.RedirectCache
	clock            ports.Clock
	operationTimeout time.Duration
	activeTTL        time.Duration
	inactiveTTL      time.Duration
}

func NewRedirectCacheRefresher(
	cache ports.RedirectCache,
	clock ports.Clock,
	config RedirectCacheConfig,
) (RedirectCacheRefresher, error) {
	if err := validateRedirectCacheConfig(config); err != nil {
		return RedirectCacheRefresher{}, err
	}

	return RedirectCacheRefresher{
		cache:            cache,
		clock:            clock,
		operationTimeout: config.OperationTimeout,
		activeTTL:        config.ActiveTTL,
		inactiveTTL:      config.InactiveTTL,
	}, nil
}

func (r RedirectCacheRefresher) Refresh(
	ctx context.Context,
	link domain.Link,
) error {
	mapping, err := domain.RedirectMappingFromLink(link)
	if err != nil {
		return err
	}

	ttl, shouldCache := redirectCacheTTL(
		mapping,
		r.clock.Now(),
		r.activeTTL,
		r.inactiveTTL,
	)
	if !shouldCache {
		return nil
	}

	cacheCtx, cancel := context.WithTimeout(ctx, r.operationTimeout)
	defer cancel()

	return r.cache.PutIfNewer(cacheCtx, mapping, ttl)
}
