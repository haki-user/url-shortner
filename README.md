# TinyURL

A production-oriented URL-shortening system implemented in Go.

The repository is intentionally scaffolded without feature implementation. The goal is to implement and defend each design decision incrementally while preserving clear service ownership and domain boundaries.

## Target Workloads

- `redirectd`: latency-sensitive redirect serving.
- `linkd`: link creation and lifecycle management.
- `invalidatord`: cache-invalidation event processing.
- `securityd`: destination scanning and emergency blocking.
- `analyticsd`: lightweight click-event aggregation.

## Repository Layout

```text
api/          External OpenAPI and internal Protobuf contracts
cmd/          Independently deployable service entry points
deploy/       Local and production deployment definitions
docs/         Architecture decisions and engineering notes
internal/     Private domain, application, port, and adapter packages
migrations/   Versioned operational-database migrations
outputs/      Complete system-design document
```

Each bounded context under `internal/` follows:

```text
domain/       Entities, value objects, invariants, and domain errors
application/  Use cases and orchestration
ports/        Interfaces required by application code
adapters/     HTTP, gRPC, database, cache, and event implementations
```

Domain and application packages must not import infrastructure-specific packages.

## Development Rules

- Establish behavior with tests before adding infrastructure.
- Keep the synchronous redirect path independent of analytics and scanning.
- Make mutation APIs idempotent or version-guarded.
- Preserve aggregate invariants inside domain behavior, not handlers.
- Record consequential architecture decisions under `docs/adr/`.
- Do not add a dependency until its concrete use is clear.

## Design Reference

See [outputs/tinyurl-system-design.md](../outputs/tinyurl-system-design.md) for the complete target architecture and low-level design.
