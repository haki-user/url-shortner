# Container Deployment

## Artifact

The root `Dockerfile` builds one production image containing:

```text
/app/linkd                         HTTP service
/app/migrate                       one-shot migration command
/app/migrations/postgres/*.sql     versioned schema migrations
```

The final image runs as numeric non-root user `65532` and contains CA
certificates for TLS connections to managed Postgres and Redis.

Build it:

```powershell
docker build -t tinyurl-linkd:local .
```

Run unit tests outside the image before publishing:

```powershell
go test ./...
go vet ./...
```

## Local Container Smoke Test

The default Compose workflow still starts only dependencies:

```powershell
docker compose -f deploy/local/compose.yaml up -d
```

Add the `app` profile to build the image, run migrations, and start `linkd`:

```powershell
docker compose -f deploy/local/compose.yaml --profile app up -d --build
docker compose -f deploy/local/compose.yaml --profile app ps
```

Stop the containerized application while keeping dependencies:

```powershell
docker compose -f deploy/local/compose.yaml stop linkd
```

## Migration Contract

Run migrations once before starting a new application release:

```text
command: /app/migrate
environment:
  TINYURL_DATABASE_URL
  TINYURL_MIGRATIONS_DIR=/app/migrations/postgres
```

The runner:

- discovers migration files by numeric prefix;
- takes a Postgres advisory lock to serialize concurrent deploys;
- runs each migration in a transaction;
- records its name and SHA-256 checksum in `schema_migrations`;
- rejects an applied migration whose contents later change.

Never edit an applied migration. Add a new numbered file instead.

## Runtime Contract

The default image entrypoint is `/app/linkd`. Required production settings:

| Variable | Example |
|---|---|
| `TINYURL_STORAGE` | `postgres` |
| `TINYURL_DATABASE_URL` | Managed Postgres URL |
| `TINYURL_CACHE` | `redis` |
| `TINYURL_REDIS_URL` | Managed Redis URL |
| `TINYURL_ADDR` | `:8080` |
| `TINYURL_BASE_URL` | Public HTTPS origin |

Keep database and Redis credentials in the provider's secret manager, not in
the image or repository. Prefer TLS URLs required by the managed providers.

## Health And Rollout

Configure the platform to use:

```text
liveness:  GET /healthz
readiness: GET /readyz
port:      8080
```

Deployment order:

```text
build immutable image
        |
        v
run /app/migrate once
        |
        v
start new linkd instances
        |
        v
wait for /readyz
        |
        v
drain old instances with SIGTERM
```

`linkd` handles `SIGTERM` with bounded graceful shutdown. Postgres is required
for readiness; Redis failure degrades to Postgres fallback and does not make
the instance unready.

## Remaining Production Work

- Replace the local `X-Owner-ID` trust model with verified authentication.
- Add cache and request metrics, tracing, and alerting.
- Add a Redis circuit breaker for prolonged cache outages.
- Configure backups, retention, and point-in-time recovery for Postgres.
- Add CI image scanning and signed image publication.
