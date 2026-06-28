# ADR 0001: Postgres Persistence

## Status

Accepted

## Context

TinyURL currently stores links, idempotency records, and redirect analytics in memory. This keeps the code easy to test while learning the architecture, but data disappears on process restart.

Before deployment, the service needs durable storage for link metadata and idempotency records. Redirect analytics should also be stored durably, but analytics writes are less critical than redirect correctness.

The code already separates domain/application logic from infrastructure through ports. Persistence should preserve that boundary.

## Decision

Use Postgres as the primary persistence layer for:

- link metadata
- idempotency records
- redirect analytics events

Keep SQL inside infrastructure adapters. Domain and application packages should continue to depend on ports, not database details.

Detailed table definitions, constraints, and indexes live in `docs/schema/postgres.md`.

## Alternatives

### In-Memory Storage

In-memory storage is excellent for tests and local learning, but it is not durable and cannot support real deployment semantics.

### DynamoDB

DynamoDB can scale very well for key-value access patterns and would be a reasonable high-scale choice for link lookup. It adds more modeling decisions around secondary indexes, transactions, and analytics storage than this project needs right now.

### Cassandra Or Bigtable

Wide-column stores can handle huge redirect workloads, but they increase operational and modeling complexity. They are better suited after traffic requires horizontal write scaling beyond a relational database.

### Separate Analytics Pipeline

At very high scale, redirect analytics should likely move to an append-only event stream and analytics store. That is intentionally out of scope for the first durable implementation.

## Consequences

Postgres gives strong consistency, transactions, uniqueness constraints, foreign keys, and mature operational tooling. This fits the current service size and keeps the implementation production-shaped without overbuilding.

The next implementation step is adding Postgres-backed adapters for existing ports:

- `LinkRepository`
- `IdempotencyStore`
- `RedirectEventRecorder`

Redirect analytics will initially be stored in Postgres. If redirect volume becomes very high, the analytics recorder can later be replaced with an asynchronous event pipeline without changing the redirect use case.
