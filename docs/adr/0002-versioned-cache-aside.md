# ADR 0002: Versioned Cache-Aside Redirect Resolution

## Status

Proposed

Change this ADR to `Accepted` when the Redis-backed resolver is implemented and
enabled in runtime configuration.

## Context

Redirect traffic is expected to greatly exceed create and mutation traffic.
Reading the source-of-truth store for every redirect would increase latency,
database cost, and hot-key risk.

Links are mutable. Owners can change destination, status, and expiration.
Ordinary cache-aside loading is therefore insufficient: a slow source read can
attempt to cache an old value after a newer mutation has already refreshed or
invalidated the key.

Management reads also require the latest source-of-truth version for
`If-Match`. Applying eventual cache semantics to the repository globally would
weaken that contract.

## Decision

Use a Redis-compatible regional cache for redirect resolution with these
rules:

1. Redirects use a dedicated `LinkResolver`; management use cases continue
   using `LinkRepository`.
2. The resolver applies cache-aside behavior: cache hit first, then
   source-of-truth fallback and best-effort fill.
3. Cache entries contain a redirect-only projection: code, destination, status,
   expiration, and version.
4. Cache writes are atomic and version-conditional. An older version cannot
   replace a newer version.
5. Status and expiration are validated after every cache hit.
6. Cache timeout or failure falls back to the source of truth.
7. Redis is not a readiness dependency.
8. Successful writes initially refresh Redis best effort with the updated
   version.
9. A later transactional-outbox implementation will provide durable,
   cross-process versioned refresh.
10. Initial TTLs remain short and are clamped to natural expiration.

Detailed algorithms, payloads, failure behavior, and rollout steps live in
[Redirect Cache Design](../architecture/redirect-cache.md).

## Alternatives

### Continue Reading Postgres Directly

This is simple and strongly consistent, but does not meet the target redirect
latency, source-load, or hot-key requirements.

### Put Caching Inside `LinkRepository`

A repository decorator would require fewer constructor changes, but management
reads could receive stale versions. Redirect reads and management reads have
different consistency requirements, so they should use different ports.

### Use Only An In-Process Cache

An in-process cache is faster than Redis, but each replica warms independently,
duplicates memory, and loses all entries on restart. It may become an L1 layer
later, after the shared regional cache is measured.

### Blindly Overwrite Cache Entries

Blind writes are simpler but permit a late version `N` database read to replace
version `N+1`. This can restore stale destinations or active status after a
disable/delete operation.

### Delete Cache Keys After Mutations

Deletion alone leaves a race:

```text
slow read obtains version N
mutation commits version N+1
mutation deletes cache key
slow read writes version N into the empty key
```

Publishing or writing the newer version and rejecting older writes closes this
race.

### Make Redis A Required Dependency

Failing redirects whenever Redis is unavailable would turn an optimization into
a new availability risk. Source-of-truth fallback is preferred, protected by
timeouts, concurrency limits, and later request coalescing.

### Add Negative Caching Immediately

Negative caching protects the source store from invalid-code traffic, but it
adds creation and invalidation races. It is deferred until positive cache
correctness is established.

## Consequences

### Positive

- Popular redirects avoid source-of-truth reads.
- Management consistency remains unchanged.
- Redis outages degrade latency rather than basic correctness.
- Version checks prevent delayed fills from restoring older mappings.
- Cache values exclude owner and management-only data.
- The resolver and cache remain replaceable through ports.

### Costs

- Redirect and management reads use separate abstractions.
- Atomic version comparison requires a Redis script or transaction.
- Mutation use cases need best-effort refresh until outbox publication exists.
- Short TTLs limit but do not eliminate stale windows when refresh fails.
- Cache outage fallback must be bounded to prevent database overload.
- Metrics, serialization versioning, and operational tuning become required.

### Follow-Up

- Implement the resolver and Redis adapter behind configuration.
- Add Redis to local Compose on host port `6380`.
- Measure hit rate, latency, fallback load, and version rejections.
- Add negative caching and per-code request coalescing.
- Add transactional outbox and `invalidatord`.
- Revisit L1 in-process caching only after profiling.
