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

### Why Mutation Refresh Is Best Effort

Postgres is updated before Redis:

```text
1. validate and mutate the domain Link
2. commit the new version to Postgres
3. attempt a short Redis refresh
```

If Postgres commits version 8 but Redis refresh fails, the durable business
operation has already succeeded. Returning an API failure would misrepresent
the result:

```text
API says failure
        |
        `-- Postgres actually contains version 8
```

The client may retry an operation that already committed. We therefore return
success and tolerate bounded cache staleness:

```text
Postgres version 8 committed
Redis still has version 7
        |
        +-- TTL eventually removes version 7
        `-- next cache miss loads version 8
```

“Best effort” means:

- attempt the refresh after the database commit;
- use a short timeout;
- record refresh failures in logs and metrics;
- do not fail or roll back the committed mutation;
- use `PutIfNewer` so delayed older writes cannot replace newer cache state.

Redis must not be updated first. If Redis accepted version 8 and the later
Postgres write failed, the cache could serve state that never became
authoritative.

The production evolution is a transactional outbox:

```text
Postgres transaction
    |-- update Link to version 8
    `-- insert version-8 outbox event
                |
                v
retrying worker refreshes Redis
```

This makes propagation reliable without making Redis part of the database
transaction or the management API's availability.

### Current Implementation: Repository Decorator

The three mutation use cases already commit through `LinkRepository.Update`.
Instead of duplicating cache code in each use case, the composition root wraps
the source repository:

```text
Change status/destination/expiration
                  |
                  v
       CacheRefreshingRepository
          |                 |
          | required        | best effort
          v                 v
   Postgres repository   RedirectCacheRefresher
                              |
                              v
                       PutIfNewer in Redis
```

The decorator preserves repository behavior:

```text
Postgres Update fails
    -> return the Postgres error
    -> do not refresh Redis

Postgres Update succeeds, Redis refresh fails
    -> return success
    -> TTL bounds the stale cache entry
```

`RedirectCacheRefresher` and `CacheAsideResolver` call the same
`redirectCacheTTL` function. This keeps mutation refresh and cache-miss fill
consistent about active, inactive, and expiring mappings.

Current limitation: refresh failures are intentionally swallowed to preserve
the committed result, but they are not yet emitted as metrics. Cache-operation
metrics and structured error recording remain part of the observability
backlog.

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

## Full Object Graph: Who Accepts Whom

Build the runtime graph from the infrastructure inward:

```text
Postgres adapter
      |
      | implements LinkRepository
      v
RepositoryResolver(repository)
      |
      | implements LinkResolver
      v
CacheAsideResolver(cache, sourceResolver)
      ^                    ^
      |                    |
Redis adapter              RepositoryResolver
implements RedirectCache   passed as LinkResolver
      |
      +--------------------+
               |
               | CacheAsideResolver implements LinkResolver
               v
       RedirectLink(resolver, clock)
               |
               v
          HTTP handler
```

The corresponding construction order is:

```go
repository := postgres.NewRepository(database)
source := application.NewRepositoryResolver(repository)

cache := redis.NewRedirectCache(redisClient)
resolver := application.NewCacheAsideResolver(cache, source)

redirectLink := application.NewRedirectLink(resolver, clock)
handler := httpapi.NewHandler(createGeneratedLink, redirectLink, baseURL)
```

The names describe roles:

| Object being constructed | What it accepts | Why |
|---|---|---|
| `RepositoryResolver` | `LinkRepository` | Reads a full authoritative `Link` and projects it |
| `CacheAsideResolver` | `RedirectCache`, `LinkResolver` | Tries cache, then delegates to its source |
| `RedirectLink` | `LinkResolver`, `Clock` | Resolves and validates without knowing the data source |
| HTTP handler | Use-case objects | Translates HTTP requests into application calls |

`RedirectLink` accepts either resolver because both satisfy the same interface:

```text
                         +-> RepositoryResolver
RedirectLink(LinkResolver)
                         +-> CacheAsideResolver
```

`RepositoryResolver` does not accept `CacheAsideResolver`. It is the inner
source. `CacheAsideResolver` accepts and wraps `RepositoryResolver`.

```text
outer behavior                                      inner source
CacheAsideResolver --------------------------------> RepositoryResolver
try Redis first                                      read Postgres
```

When Redis is disabled, omit the wrapper:

```go
source := application.NewRepositoryResolver(repository)
redirectLink := application.NewRedirectLink(source, clock)
```

When Redis is enabled, insert the wrapper:

```go
source := application.NewRepositoryResolver(repository)
resolver := application.NewCacheAsideResolver(cache, source)
redirectLink := application.NewRedirectLink(resolver, clock)
```

This is the decorator pattern: one `LinkResolver` wraps another
`LinkResolver`, adding caching without changing the wrapped resolver or the
caller.

### Current Progress

```text
[done] RedirectMapping
[done] LinkResolver interface
[done] RepositoryResolver
[now ] RedirectLink uses LinkResolver
[next] RedirectCache interface
[later] Redis adapter and CacheAsideResolver
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

## Redis Vocabulary For This Project

Redis is a separate server that stores data under keys. Our application talks
to it over the network.

```text
key
tinyurl:redirect:v1:abc123

value (a Redis hash)
destination = https://example.com
status      = active
version     = 8
expires_at  = 1780000000000
```

A Redis hash is similar to a small JavaScript object stored under one key:

```js
{
  destination: "https://example.com",
  status: "active",
  version: 8
}
```

The important terms are:

| Term | Meaning here |
|---|---|
| Key | Redis address derived from the short code |
| Hash | Named fields stored under that key |
| Cache hit | Redis contains a usable mapping |
| Cache miss | The key does not exist |
| TTL | How long Redis keeps the key before deleting it automatically |
| Expiration | The moment the TTL reaches zero |
| Eviction | Redis removes data to reclaim memory |
| Source of truth | Postgres, where durable link state lives |

TTL means **time to live**. For example:

```text
write key with TTL = 60 seconds
          |
          +-- 20 seconds later: key exists; TTL is about 40 seconds
          |
          `-- 60 seconds later: Redis automatically removes the key
```

We use TTL for three reasons:

1. Redis memory is finite, so unused mappings should not live forever.
2. If cache refresh fails after a link changes, stale data disappears
   automatically.
3. Restarting or clearing Redis remains harmless because Postgres can rebuild
   every mapping.

TTL is not the same as a link's business expiration:

```text
cache TTL:       when Redis should forget its temporary copy
link expiresAt:  when the short link must stop redirecting
```

If a link expires in 10 seconds, we must not cache it for 60 seconds:

```text
configured cache TTL = 60 seconds
link lifetime left   = 10 seconds
actual Redis TTL     = min(60, 10) = 10 seconds
```

Even before the TTL ends, `RedirectLink` checks `expiresAt`. TTL controls
storage lifetime; domain validation controls whether redirecting is allowed.

### Exactly Which Mappings We Cache

The cache is not limited to active links:

| Stored link state | Cache it? | Initial TTL | Redirect allowed? |
|---|---|---:|---|
| Active, no expiration | Yes | 60 seconds | Yes |
| Active, expires in 10 seconds | Yes | 10 seconds | Yes, until expiration |
| Disabled, not expired | Yes | 30 seconds | No |
| Deleted, not expired | Yes | 30 seconds | No |
| Already expired | No | None | No |

Disabled and deleted mappings are cached so repeated requests for unavailable
links do not repeatedly hit Postgres. `RedirectLink` checks the cached status
and refuses the redirect.

```text
cached does not mean redirectable

cache contains mapping
        |
        v
RedirectLink checks status and expiresAt
        |
        +-- allowed -> redirect
        `-- denied  -> unavailable
```

### Why Already-Expired Links Are Not Cached

The redirect cache stores mappings that may still be useful for redirect
resolution. An already-expired link has no positive lifetime remaining:

```text
expiresAt <= now
        |
        v
remaining lifetime <= 0
        |
        v
do not write a positive cache entry
```

Giving it the normal active TTL would retain a mapping after its business
lifetime ended. `RedirectLink` would still reject it because it checks
`expiresAt`, so this would not cause an incorrect redirect, but the entry would
consume memory despite having no remaining redirect lifetime.

There is a tradeoff: repeated requests for an expired code can repeatedly read
Postgres. Solving that cleanly requires **negative caching**, where Redis stores
an explicit short-lived result such as `expired` or `not found`, rather than
pretending an expired mapping is a normal positive entry. Negative caching is
deferred until positive-cache correctness is established because creation,
reactivation, and invalidation rules become more complicated.

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

These numbers are starting values, not universal constants:

```text
longer TTL
    + higher cache-hit rate
    + fewer Postgres reads
    - stale mappings can remain longer after a failed refresh

shorter TTL
    + stale mappings self-correct sooner
    - more cache misses and Postgres reads
```

Sixty seconds is a conservative initial active TTL: popular links remain hot
while stale state is naturally bounded to about one minute if mutation refresh
fails. Thirty seconds is shorter for disabled mappings because reactivation is
plausible and serving a stale disabled result harms availability. Deleted
mappings initially share the inactive policy for simplicity, although they
could use a longer TTL because deletion is terminal in this domain.

The values must eventually be tuned from production evidence:

```text
cache hit ratio
Postgres read QPS
Redis memory usage and eviction rate
mutation frequency
stale-result observations
p95/p99 redirect latency
```

At larger scale, one fixed TTL may be wasteful. Possible refinements include:

- longer TTLs plus reliable event-driven refresh or invalidation;
- TTL jitter so many keys do not expire simultaneously;
- different TTLs for active, disabled, and deleted states;
- popularity-aware TTLs;
- bounded negative caching for expired or missing codes.

TTL jitter changes, for example, a fixed 60-second TTL into a range such as
54-66 seconds. This reduces synchronized expiration bursts and the resulting
Postgres load spike.

For expiring links:

```text
actual TTL = min(configured TTL, expiresAt - now)
```

### Interview Answer: Why 60 Seconds And 30 Seconds?

> I would not present those values as inherently correct. They are conservative
> launch defaults. A 60-second active TTL gives useful cache reuse while
> bounding stale state when best-effort refresh fails. Disabled links use 30
> seconds because they may be reactivated, so a stale unavailable response
> should self-correct sooner. The actual TTL is capped by the link's remaining
> lifetime. I would add jitter and tune the values using hit ratio, database
> load, mutation rate, Redis memory, and stale-result metrics.

### TTL Policy By Delivery Phase

TinyURL is read-heavy, so reliable invalidation should eventually allow much
longer TTLs:

```text
redirect reads >>> management writes
```

The initial TTL remains conservative because mutation refresh is only best
effort. If that refresh fails, TTL is the mechanism that eventually removes
stale state.

| Phase | Refresh guarantee | Illustrative TTL policy |
|---|---|---|
| Initial | Best-effort refresh after commit | Active 60s; inactive 30s |
| Reliable invalidation | Transactional outbox with retry | Active 5-15m with jitter; tune inactive states separately |
| Production tuning | Measured behavior | Choose from hit ratio, DB load, stale reads, memory, and latency |

These later values are illustrative rather than promises. The rule is:

```text
rare writes + reliable invalidation -> longer TTL is reasonable
rare writes + best-effort refresh   -> TTL must bound stale-state risk
```

Backlog:

- persist link-change events in a transactional outbox with the Postgres write;
- process outbox events with retry and idempotent, versioned Redis updates;
- add TTL jitter to avoid synchronized expiration bursts;
- split disabled and terminal deleted TTL policies;
- retune TTLs after invalidation reliability and production metrics exist.

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

## Synchronous Cache Calls And "Best Effort"

The initial cache-aside resolver performs its operations synchronously:

```text
Redis Get (wait at most 25ms)
        |
        +-- hit -> return mapping
        |
        `-- miss/error
                |
                v
             Postgres
                |
                v
Redis PutIfNewer (wait at most 25ms)
                |
                v
          return mapping
```

“Best effort” describes error semantics, not concurrency:

```text
Redis write fails -> keep the successful Postgres result
```

It does not currently mean:

```go
go cache.PutIfNewer(...)
```

An unbounded goroutine per cache miss would introduce several problems:

- the HTTP request context may be cancelled before the goroutine finishes;
- a miss burst could create an unbounded number of goroutines;
- shutdown would need to track and drain outstanding writes;
- errors, retries, and metrics would become harder to observe;
- the process could exit before queued writes reach Redis.

The initial policy is therefore:

```text
synchronous operation + strict timeout + cache error does not fail request
```

If cache-fill latency becomes material, move writes to a bounded worker pool
with its own lifecycle and queue limits. Do not create detached per-request
goroutines.

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

## Composition Root And Dependency Injection

`cmd/linkd/main.go` is the composition root. It knows which concrete
infrastructure implementations the process uses and connects them together.

```text
main constructs concrete dependencies
        |
        v
application receives interfaces
```

Example:

```go
linkResolver := application.NewRepositoryResolver(storage.repository)
redirectLink := application.NewRedirectLink(linkResolver, clock)
```

`RedirectLink` does not construct or depend directly on Postgres, Redis, or an
in-memory repository. It depends on the `LinkResolver` capability.

`RepositoryResolver` accepts a `LinkRepository` because its responsibility is
to translate the repository's full `Link` aggregate into the smaller
`RedirectMapping` required by the redirect use case.

```text
LinkRepository
    |
    | FindByCode returns Link
    v
RepositoryResolver
    |
    | converts Link to RedirectMapping
    v
LinkResolver contract
```

Any implementation of `LinkRepository`, such as the Postgres or in-memory
adapter, can be injected into `RepositoryResolver`. Redis is intentionally not
a `LinkRepository`: it is an optional cache, stores only `RedirectMapping`,
and is not the source of truth. It implements the separate `RedirectCache`
contract.

Later, the composition root can wrap the source resolver without changing
`RedirectLink`:

```go
source := application.NewRepositoryResolver(repository)
resolver := application.NewCacheAsideResolver(cache, source)
redirectLink := application.NewRedirectLink(resolver, clock)
```

The dependency direction remains:

```text
main -> concrete implementations
application -> interfaces
domain -> no infrastructure
```

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
