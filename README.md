# TinyURL

A production-oriented URL-shortening system implemented in Go.

The repository is implemented incrementally so each design decision can be
understood and defended while preserving clear service ownership and domain
boundaries.

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

## Local Smoke Test

Run the service:

```powershell
& "C:\Program Files\Go\bin\go.exe" run ./cmd/linkd
```

In another PowerShell window, create a short link:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/v1/links `
  -ContentType "application/json" `
  -Body '{"destination":"https://example.com","ownerId":"owner-1"}'
```

The first generated short URL should be:

```text
http://localhost:8080/1
```

Verify the redirect without following it:

```powershell
Invoke-WebRequest http://localhost:8080/1 -MaximumRedirection 0 -SkipHttpErrorCheck
```

Expected result:

```text
StatusCode: 302
Location: https://example.com
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `TINYURL_STORAGE` | `memory` | Storage adapter: `memory` or `postgres` |
| `TINYURL_DATABASE_URL` | none | Postgres connection URL; required with Postgres storage |
| `TINYURL_ADDR` | `:8080` | Address on which the HTTP server listens |
| `TINYURL_BASE_URL` | `http://localhost:8080` | Public URL used when returning short links |
| `TINYURL_SHUTDOWN_TIMEOUT` | `10s` | Maximum time allowed for graceful HTTP shutdown |

Use [.env.example](.env.example) as a local configuration template. The Go
service reads operating-system environment variables and does not automatically
load a `.env` file. Development tooling or the shell must load those values
before starting the service.

## Health Endpoints

- `GET /healthz` is a liveness check. It returns `200` while the process can
  serve HTTP and does not check external dependencies.
- `GET /readyz` is a readiness check. It returns `200` when required storage is
  available and `503` when the service should not receive traffic.

Readiness checks have a two-second timeout. Health responses use
`Cache-Control: no-store` so infrastructure does not reuse stale probe results.
With Postgres storage, stopping Postgres leaves `/healthz` healthy while
`/readyz` reports `503`; readiness recovers after Postgres reconnects.

## Link Management

Read the current owner-facing state and version of a link:

```http
GET /v1/links/{code}
X-Owner-ID: owner-1
```

A successful response includes status, timestamps, expiration, and version,
plus an `ETag` such as `"1"`. The response does not expose the owner ID. Missing
identity returns `401`; a missing link or ownership mismatch returns `404`.

`X-Owner-ID` is only a local-development stand-in. Production identity must
come from verified authentication or a trusted gateway.

Change a link's lifecycle status using the version returned by the management
read:

```http
PATCH /v1/links/{code}
X-Owner-ID: owner-1
If-Match: "1"
Content-Type: application/json

{"status":"disabled"}
```

Supported targets are `active`, `disabled`, and `deleted`. A successful update
returns the updated resource and its new `ETag`. Missing `If-Match` returns
`428`, a stale version returns `412`, and an invalid lifecycle transition
returns `409`. Deletion is terminal.

The same endpoint can update the destination:

```http
PATCH /v1/links/{code}
X-Owner-ID: owner-1
If-Match: "2"
Content-Type: application/json

{"destination":"https://example.com/new"}
```

Each PATCH currently accepts exactly one mutable field. Invalid destinations
return `400`; unchanged destinations or updates to deleted links return `409`.
A successful update changes future redirects immediately and returns the next
resource version.

## Local Postgres

Postgres runs through Docker Compose for local development. The Go service still runs directly on your machine until we containerize the app.

See [Local Postgres Development](docs/development/postgres.md).

## Learning Reference

See the [Go Learning Handbook](docs/learning/go/README.md) for the language mental model, project context, detailed explanations, cheat sheet, and question log built alongside this project.
