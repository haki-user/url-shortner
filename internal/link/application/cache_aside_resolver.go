package application

import (
	"context"
	"errors"
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

type RedirectCacheMetrics interface {
	RecordCacheGet(result string, duration time.Duration)
	RecordSourceLookup(result string, duration time.Duration)
	RecordCachePut(result string, duration time.Duration)
}

type RedirectCacheOption func(*CacheAsideResolver)

func WithRedirectCacheMetrics(metrics RedirectCacheMetrics) RedirectCacheOption {
	return func(r *CacheAsideResolver) {
		r.metrics = metrics
	}
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
	metrics          RedirectCacheMetrics
}

func NewCacheAsideResolver(
	cache ports.RedirectCache,
	source ports.LinkResolver,
	clock ports.Clock,
	config RedirectCacheConfig,
	options ...RedirectCacheOption,
) (CacheAsideResolver, error) {
	if err := validateRedirectCacheConfig(config); err != nil {
		return CacheAsideResolver{}, err
	}

	resolver := CacheAsideResolver{
		cache:            cache,
		source:           source,
		clock:            clock,
		operationTimeout: config.OperationTimeout,
		activeTTL:        config.ActiveTTL,
		inactiveTTL:      config.InactiveTTL,
	}

	for _, option := range options {
		option(&resolver)
	}

	return resolver, nil
}

func (r CacheAsideResolver) Resolve(
	ctx context.Context,
	code string,
) (domain.RedirectMapping, error) {
	cacheCtx, cancelCacheRead := context.WithTimeout(
		ctx,
		r.operationTimeout,
	)

	cacheReadStartedAt := time.Now()
	mapping, err := r.cache.Get(cacheCtx, code)
	cancelCacheRead()

	if err == nil {
		r.recordCacheGet("hit", time.Since(cacheReadStartedAt))
		return mapping, nil
	}

	cacheGetResult := "error"
	if errors.Is(err, ports.ErrRedirectCacheMiss) {
		cacheGetResult = "miss"
	}
	r.recordCacheGet(cacheGetResult, time.Since(cacheReadStartedAt))

	sourceStartedAt := time.Now()
	mapping, err = r.source.Resolve(ctx, code)
	if err != nil {
		r.recordSourceLookup("error", time.Since(sourceStartedAt))
		return domain.RedirectMapping{}, err
	}
	r.recordSourceLookup("success", time.Since(sourceStartedAt))

	ttl, shouldCache := redirectCacheTTL(
		mapping,
		r.clock.Now(),
		r.activeTTL,
		r.inactiveTTL,
	)
	if !shouldCache {
		r.recordCachePut("skipped", 0)
		return mapping, nil
	}

	cacheCtx, cancelCacheWrite := context.WithTimeout(
		ctx,
		r.operationTimeout,
	)
	defer cancelCacheWrite()

	cacheWriteStartedAt := time.Now()
	if err := r.cache.PutIfNewer(cacheCtx, mapping, ttl); err != nil {
		r.recordCachePut("error", time.Since(cacheWriteStartedAt))
		return mapping, nil
	}

	r.recordCachePut("success", time.Since(cacheWriteStartedAt))

	return mapping, nil
}

func (r CacheAsideResolver) recordCacheGet(result string, duration time.Duration) {
	if r.metrics == nil {
		return
	}

	r.metrics.RecordCacheGet(result, duration)
}

func (r CacheAsideResolver) recordSourceLookup(result string, duration time.Duration) {
	if r.metrics == nil {
		return
	}

	r.metrics.RecordSourceLookup(result, duration)
}

func (r CacheAsideResolver) recordCachePut(result string, duration time.Duration) {
	if r.metrics == nil {
		return
	}

	r.metrics.RecordCachePut(result, duration)
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
