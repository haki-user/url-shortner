# Redirect Caching Mental Model

## What We Are Changing

Today, redirect and management requests both read through `LinkRepository`:

```text
                         CURRENT SYSTEM

Owner request  -> Management use case --+
                                         |
Visitor        -> RedirectLink -----------+-> LinkRepository -> Postgres
```

This is correct, but every redirect reaches Postgres.

We will add Redis only to the redirect path:

```text
                          SYSTEM WITH CACHE

Owner request  -> Management use case -> LinkRepository -> Postgres

Visitor -> RedirectLink -> LinkResolver
                              |
                              v
                     CacheAsideResolver
                         |         |
                         |         +-> SourceResolver -> LinkRepository -> Postgres
                         |
                         +-> RedirectCache -> Redis
```

The management path remains strongly consistent. Redis never becomes the
source of truth.

## HLD: Runtime Components

```text
 +------------------+       +------------------+
 | Owner / API user |       |     Visitor      |
 +--------+---------+       +--------+---------+
          |                          |
          v                          v
 +------------------+       +------------------+
 | Management APIs  |       |  Redirect API    |
 +--------+---------+       +--------+---------+
          |                          |
          |                          v
          |                 +------------------+
          |                 | Cache-aside      |
          |                 | resolver         |
          |                 +----+--------+----+
          |                      |        |
          |              fallback|        |first
          |                      v        v
          |                 +---------+  +---------+
          +---------------->|Postgres |  |  Redis  |
                            +---------+  +---------+
                                 ^
                                 |
                         source of truth
```

The resolver attempts Redis first. On a cache miss, timeout, or connection
error, it reads Postgres.

### Responsibilities

| Component | Knows about | Does not know about |
|---|---|---|
| `RedirectLink` | Redirect rules and result | Redis or Postgres |
| `LinkResolver` | How to obtain a redirect mapping | HTTP |
| `CacheAsideResolver` | Cache-first fallback algorithm | Redis commands |
| `RedirectCache` | Cache capability contract | Postgres |
| Redis adapter | Redis commands and serialization | Business workflows |
| `LinkRepository` | Persistent link operations | Caching policy |
| `main.go` | Which implementations to connect | Business rules |

## HLD: Read Flow

```text
Visitor
   |
   v
RedirectLink.Execute(code)
   |
   v
LinkResolver.Resolve(code)
   |
   +---- Redis HIT -----------------------------+
   |                                            |
   +---- Redis MISS / ERROR                     |
              |                                 |
              v                                 |
          Postgres                              |
              |                                 |
              v                                 |
       build RedirectMapping                    |
              |                                 |
              +-> PutIfNewer in Redis            |
              |    best effort                  |
              +---------------------------------+
                                                |
                                                v
                                      validate mapping
                                                |
                              +-----------------+----------------+
                              |                                  |
                         active and valid               unavailable/expired
                              |                                  |
                              v                                  v
                         302 redirect                            404
```

Redis failure increases Postgres traffic, but it does not make the service
unready.

## HLD: Mutation Flow

```text
Owner changes link
       |
       v
Read current link from Postgres
       |
       v
Apply domain mutation
version 7 becomes version 8
       |
       v
Commit version 8 to Postgres
       |
       +-> failed: return error; do not touch Redis
       |
       +-> succeeded
              |
              v
       PutIfNewer(version 8)
       short, best-effort call
              |
              +-> success: future redirects see version 8
              |
              +-> failure: keep database success;
                           short TTL bounds stale data
```

Later, a transactional outbox will replace this best-effort refresh.

## LLD: Object Connections

### Current

```text
RedirectLink
  repository ports.LinkRepository
  clock      ports.Clock
```

### After The Change

```text
RedirectLink
  resolver ports.LinkResolver
  clock    ports.Clock
      |
      v
CacheAsideResolver
  cache  ports.RedirectCache
  source ports.LinkResolver
      |             |
      v             v
Redis adapter   RepositoryResolver
                    |
                    v
              ports.LinkRepository
```

The important pattern is composition:

```text
RepositoryResolver implements LinkResolver
CacheAsideResolver  implements LinkResolver

RedirectLink accepts either one through the same interface.
```

When caching is disabled:

```text
RedirectLink -> RepositoryResolver -> Postgres
```

When caching is enabled:

```text
RedirectLink -> CacheAsideResolver -> Redis
                                  -> RepositoryResolver -> Postgres
```

## LLD: Contracts

```go
type LinkResolver interface {
	Resolve(
		ctx context.Context,
		code string,
	) (domain.RedirectMapping, error)
}

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
```

`LinkResolver` hides the data source from `RedirectLink`.

`RedirectCache` hides Redis commands from application code.

## LLD: Cached Data

```go
type RedirectMapping struct {
	Code        string
	Destination string
	Status      LinkStatus
	ExpiresAt   *time.Time
	Version     uint64
}
```

Redis key:

```text
tinyurl:redirect:v1:{code}
```

Stored fields:

```text
destination = https://example.com/path
status      = active
expires_at  = 1780000000000
version     = 8
```

Owner ID, idempotency records, analytics, and management timestamps are not
needed to make a redirect decision, so they are not cached.

## LLD: Cache-Aside Algorithm

```text
Resolve(ctx, code):
    1. derive a 25ms cache context from ctx
    2. cache.Get(code)

    3. if hit:
           return cached mapping

    4. if miss or cache failure:
           source.Resolve(code)

    5. calculate TTL
    6. cache.PutIfNewer(mapping, TTL), best effort
    7. return source mapping
```

`RedirectLink` validates status and expiration after resolution. A cache key
being present never automatically means redirecting is allowed.

## LLD: Atomic Version Check

Redis must compare and write as one operation:

```text
stored version   = 8
incoming version = 7

7 < 8 -> reject incoming value
```

Why:

```text
Slow redirect reads Postgres version 7
Owner commits and caches version 8
Slow redirect finally tries to cache version 7
PutIfNewer rejects version 7
Version 8 remains cached
```

The Redis adapter will use a Lua script so another command cannot run between
the version check and write.

## Timeouts And TTL

```text
HTTP request context
    |
    +-> Redis operation context: 25ms
    |
    +-> Postgres operation: remaining request deadline
```

Initial TTL policy:

| Mapping | TTL |
|---|---:|
| Active | 60 seconds |
| Disabled or deleted | 30 seconds |
| Already expired | Do not positively cache |

For expiring links:

```text
actual TTL = min(configured TTL, expiresAt - now)
```

## Failure Decisions

| Event | Result |
|---|---|
| Redis hit | Use mapping after validation |
| Redis miss | Read Postgres and fill Redis |
| Redis timeout/error | Read Postgres and record metric |
| Redis fill failure | Return Postgres result |
| Redis and Postgres unavailable | Return service error |
| Mapping inactive or expired | Never redirect |
| Older cache write arrives | Reject it |

Postgres remains part of readiness. Redis does not.

## Where The Code Will Go

```text
internal/link/
|-- domain/
|   `-- redirect_mapping.go
|-- ports/
|   |-- link_resolver.go
|   `-- redirect_cache.go
|-- application/
|   |-- redirect_link.go
|   |-- repository_resolver.go
|   `-- cache_aside_resolver.go
`-- adapters/
    `-- redis/
        `-- redirect_cache.go

internal/config/config.go        cache configuration
cmd/linkd/main.go                dependency wiring
compose.yaml                     local Redis
```

## Implementation Sequence

```text
1. RedirectMapping
        |
2. LinkResolver
        |
3. RepositoryResolver
        |
4. RedirectLink uses LinkResolver
        |
   current behavior still works
        |
5. RedirectCache port
        |
6. Redis adapter + PutIfNewer
        |
7. CacheAsideResolver
        |
8. Configuration and wiring
        |
9. Best-effort mutation refresh
        |
10. Metrics and verification
```

Each step keeps the service runnable. Redis appears only after the resolver
boundary works without it.

## Mind Map

```text
REDIRECT CACHING
|
|-- Purpose
|   `-- reduce Postgres reads and redirect latency
|
|-- Truth
|   `-- Postgres
|
|-- Read path
|   |-- Redis hit -> validate -> respond
|   `-- miss/error -> Postgres -> fill -> respond
|
|-- Write path
|   |-- commit Postgres first
|   `-- refresh Redis best effort
|
|-- Correctness
|   |-- status validation
|   |-- expiration validation
|   |-- versioned mappings
|   `-- atomic PutIfNewer
|
|-- Resilience
|   |-- short Redis timeout
|   |-- Postgres fallback
|   |-- bounded TTL
|   `-- Redis excluded from readiness
|
`-- Later
    |-- negative caching
    |-- per-code request coalescing
    |-- transactional outbox
    `-- multi-region invalidation
```

## One-Sentence Test

If Redis vanished during a redirect, the request should still be correct by
reading Postgres; it may only be slower.
