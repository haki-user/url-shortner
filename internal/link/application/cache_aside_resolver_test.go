package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type cacheAsideCacheFake struct {
	getCalls int
	getValue domain.RedirectMapping
	getErr   error

	putCalls   int
	putMapping domain.RedirectMapping
	putTTL     time.Duration
	putErr     error
}

func (f *cacheAsideCacheFake) Get(
	ctx context.Context,
	code string,
) (domain.RedirectMapping, error) {
	f.getCalls++
	return f.getValue, f.getErr
}

func (f *cacheAsideCacheFake) PutIfNewer(
	ctx context.Context,
	mapping domain.RedirectMapping,
	ttl time.Duration,
) error {
	f.putCalls++
	f.putMapping = mapping
	f.putTTL = ttl
	return f.putErr
}

type cacheAsideSourceFake struct {
	calls   int
	mapping domain.RedirectMapping
	err     error
}

func (f *cacheAsideSourceFake) Resolve(
	ctx context.Context,
	code string,
) (domain.RedirectMapping, error) {
	f.calls++
	return f.mapping, f.err
}

type cacheAsideClockFake struct {
	now time.Time
}

func (f cacheAsideClockFake) Now() time.Time {
	return f.now
}

type cacheAsideMetricsFake struct {
	cacheGetResults     []string
	sourceLookupResults []string
	cachePutResults     []string
}

func (f *cacheAsideMetricsFake) RecordCacheGet(result string, duration time.Duration) {
	f.cacheGetResults = append(f.cacheGetResults, result)
}

func (f *cacheAsideMetricsFake) RecordSourceLookup(result string, duration time.Duration) {
	f.sourceLookupResults = append(f.sourceLookupResults, result)
}

func (f *cacheAsideMetricsFake) RecordCachePut(result string, duration time.Duration) {
	f.cachePutResults = append(f.cachePutResults, result)
}

func TestCacheAsideResolverHitReturnsCachedMapping(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	cached := mustCacheAsideMapping(t, "https://cached.example", domain.Active, nil, 8)

	cache := &cacheAsideCacheFake{getValue: cached}
	source := &cacheAsideSourceFake{}

	resolver := mustCacheAsideResolver(t, cache, source, now)

	result, err := resolver.Resolve(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Version() != 8 {
		t.Fatalf("expected cached version 8, got %d", result.Version())
	}

	if source.calls != 0 {
		t.Fatalf("expected source not to be called, got %d calls", source.calls)
	}

	if cache.putCalls != 0 {
		t.Fatalf("expected cache not to be written on hit, got %d writes", cache.putCalls)
	}
}

func TestCacheAsideResolverRecordsMetrics(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	sourceMapping := mustCacheAsideMapping(
		t,
		"https://source.example",
		domain.Active,
		nil,
		9,
	)

	cache := &cacheAsideCacheFake{
		getErr: ports.ErrRedirectCacheMiss,
	}
	source := &cacheAsideSourceFake{mapping: sourceMapping}
	metrics := &cacheAsideMetricsFake{}

	resolver := mustCacheAsideResolverWithMetrics(t, cache, source, now, metrics)

	_, err := resolver.Resolve(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertStringSlice(t, metrics.cacheGetResults, []string{"miss"})
	assertStringSlice(t, metrics.sourceLookupResults, []string{"success"})
	assertStringSlice(t, metrics.cachePutResults, []string{"success"})
}

func TestCacheAsideResolverMissOrFailureFallsBackAndFillsCache(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		getErr error
		putErr error
	}{
		{
			name:   "cache miss",
			getErr: ports.ErrRedirectCacheMiss,
		},
		{
			name:   "cache failure",
			getErr: errors.New("redis unavailable"),
		},
		{
			name:   "cache fill failure remains successful",
			getErr: ports.ErrRedirectCacheMiss,
			putErr: errors.New("redis write failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceMapping := mustCacheAsideMapping(
				t,
				"https://source.example",
				domain.Active,
				nil,
				9,
			)

			cache := &cacheAsideCacheFake{
				getErr: tt.getErr,
				putErr: tt.putErr,
			}
			source := &cacheAsideSourceFake{mapping: sourceMapping}

			resolver := mustCacheAsideResolver(t, cache, source, now)

			result, err := resolver.Resolve(context.Background(), "abc123")
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if result.Version() != 9 {
				t.Fatalf("expected source version 9, got %d", result.Version())
			}

			if source.calls != 1 {
				t.Fatalf("expected one source call, got %d", source.calls)
			}

			if cache.putCalls != 1 {
				t.Fatalf("expected one cache fill, got %d", cache.putCalls)
			}

			if cache.putTTL != 60*time.Second {
				t.Fatalf("expected TTL 60s, got %s", cache.putTTL)
			}

			if cache.putMapping.Version() != 9 {
				t.Fatalf(
					"expected cached version 9, got %d",
					cache.putMapping.Version(),
				)
			}
		})
	}
}

func TestCacheAsideResolverDoesNotCacheExpiredMapping(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Second)

	mapping := mustCacheAsideMapping(
		t,
		"https://expired.example",
		domain.Active,
		&expiredAt,
		4,
	)

	cache := &cacheAsideCacheFake{
		getErr: ports.ErrRedirectCacheMiss,
	}
	source := &cacheAsideSourceFake{mapping: mapping}

	resolver := mustCacheAsideResolver(t, cache, source, now)

	result, err := resolver.Resolve(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Version() != 4 {
		t.Fatalf("expected source version 4, got %d", result.Version())
	}

	if cache.putCalls != 0 {
		t.Fatalf("expected expired mapping not to be cached, got %d writes", cache.putCalls)
	}
}

func TestCacheAsideResolverReturnsSourceError(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	sourceErr := errors.New("postgres unavailable")

	cache := &cacheAsideCacheFake{
		getErr: ports.ErrRedirectCacheMiss,
	}
	source := &cacheAsideSourceFake{err: sourceErr}

	resolver := mustCacheAsideResolver(t, cache, source, now)

	_, err := resolver.Resolve(context.Background(), "abc123")
	if !errors.Is(err, sourceErr) {
		t.Fatalf("expected source error %v, got %v", sourceErr, err)
	}

	if cache.putCalls != 0 {
		t.Fatalf("expected no cache fill after source failure, got %d", cache.putCalls)
	}
}

func mustCacheAsideResolver(
	t *testing.T,
	cache ports.RedirectCache,
	source ports.LinkResolver,
	now time.Time,
) CacheAsideResolver {
	t.Helper()

	return mustCacheAsideResolverWithMetrics(t, cache, source, now, nil)
}

func mustCacheAsideResolverWithMetrics(
	t *testing.T,
	cache ports.RedirectCache,
	source ports.LinkResolver,
	now time.Time,
	metrics RedirectCacheMetrics,
) CacheAsideResolver {
	t.Helper()

	options := []RedirectCacheOption{}
	if metrics != nil {
		options = append(options, WithRedirectCacheMetrics(metrics))
	}

	resolver, err := NewCacheAsideResolver(
		cache,
		source,
		cacheAsideClockFake{now: now},
		RedirectCacheConfig{
			OperationTimeout: 25 * time.Millisecond,
			ActiveTTL:        60 * time.Second,
			InactiveTTL:      30 * time.Second,
		},
		options...,
	)
	if err != nil {
		t.Fatalf("expected resolver setup to succeed, got %v", err)
	}

	return resolver
}

func assertStringSlice(t *testing.T, actual []string, expected []string) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("expected %v, got %v", expected, actual)
		}
	}
}

func mustCacheAsideMapping(
	t *testing.T,
	rawDestination string,
	status domain.LinkStatus,
	expiresAt *time.Time,
	version uint64,
) domain.RedirectMapping {
	t.Helper()

	destination, err := domain.NewDestinationURL(rawDestination)
	if err != nil {
		t.Fatalf("expected destination setup to succeed, got %v", err)
	}

	mapping, err := domain.NewRedirectMapping(
		"abc123",
		destination,
		status,
		expiresAt,
		version,
	)
	if err != nil {
		t.Fatalf("expected mapping setup to succeed, got %v", err)
	}

	return mapping
}
