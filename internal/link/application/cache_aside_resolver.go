package application

import (
	"context"
	"fmt"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type RedirectCacheConfig struct {
	OperationTimeout time.Duration
	ActiveTTL        time.Duration
	InactiveTTL      time.Duration
}

// CacheAsideResolver reads redirect mappings from cache and falls back to an
// authoritative resolver on cache miss or failure.
type CacheAsideResolver struct {
	cache            ports.RedirectCache
	source           ports.LinkResolver
	clock            ports.Clock
	operationTimeout time.Duration
	activeTTL        time.Duration
	inactiveTTL      time.Duration
}

func NewCacheAsideResolver(
	cache ports.RedirectCache,
	source ports.LinkResolver,
	clock ports.Clock,
	config RedirectCacheConfig,
) (CacheAsideResolver, error) {
	if err := validateRedirectCacheConfig(config); err != nil {
		return CacheAsideResolver{}, err
	}

	return CacheAsideResolver{
		cache:            cache,
		source:           source,
		clock:            clock,
		operationTimeout: config.OperationTimeout,
		activeTTL:        config.ActiveTTL,
		inactiveTTL:      config.InactiveTTL,
	}, nil
}

func (r CacheAsideResolver) Resolve(
	ctx context.Context,
	code string,
) (domain.RedirectMapping, error) {
	cacheCtx, cancelCacheRead := context.WithTimeout(
		ctx,
		r.operationTimeout,
	)

	mapping, err := r.cache.Get(cacheCtx, code)
	cancelCacheRead()

	if err == nil {
		return mapping, nil
	}

	mapping, err = r.source.Resolve(ctx, code)
	if err != nil {
		return domain.RedirectMapping{}, err
	}

	ttl, shouldCache := redirectCacheTTL(
		mapping,
		r.clock.Now(),
		r.activeTTL,
		r.inactiveTTL,
	)
	if !shouldCache {
		return mapping, nil
	}

	cacheCtx, cancelCacheWrite := context.WithTimeout(
		ctx,
		r.operationTimeout,
	)
	defer cancelCacheWrite()

	_ = r.cache.PutIfNewer(cacheCtx, mapping, ttl)

	return mapping, nil
}

func validateRedirectCacheConfig(config RedirectCacheConfig) error {
	if config.OperationTimeout <= 0 {
		return fmt.Errorf("cache operation timeout must be positive")
	}

	if config.ActiveTTL <= 0 {
		return fmt.Errorf("active cache TTL must be positive")
	}

	if config.InactiveTTL <= 0 {
		return fmt.Errorf("inactive cache TTL must be positive")
	}

	return nil
}

func redirectCacheTTL(
	mapping domain.RedirectMapping,
	now time.Time,
	activeTTL time.Duration,
	inactiveTTL time.Duration,
) (time.Duration, bool) {
	ttl := inactiveTTL
	if mapping.Status() == domain.Active {
		ttl = activeTTL
	}

	expiresAt := mapping.ExpiresAt()
	if expiresAt == nil {
		return ttl, true
	}

	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return 0, false
	}

	if remaining < ttl {
		return remaining, true
	}

	return ttl, true
}

var _ ports.LinkResolver = CacheAsideResolver{}
