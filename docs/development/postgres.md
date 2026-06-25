# Local Postgres Development

This project runs Postgres in Docker Compose for local development.

The Go service should still run directly on your machine for now:

- faster edit-run-test loop
- easier debugger/logging experience
- no app container needed until deployment packaging

Only the database runs in a container.

## Prerequisites

- Docker Desktop
- PowerShell

## Start Postgres

From the repository root:

```powershell
docker compose -f deploy/local/compose.yaml up -d
```

Check that Postgres is healthy:

```powershell
docker compose -f deploy/local/compose.yaml ps
```

## Run Migrations

Run the first schema migration with `psql` inside the Postgres container:

```powershell
docker compose -f deploy/local/compose.yaml exec postgres `
  psql -U tinyurl -d tinyurl -f /migrations/0001_init.sql
```

For now, migrations are manual SQL files. This keeps the learning path simple.

Later, when there are multiple migrations and environments, add a migration runner such as `golang-migrate`, `goose`, or a small internal migration command.

## Connect To The Database

Open a `psql` shell:

```powershell
docker compose -f deploy/local/compose.yaml exec postgres psql -U tinyurl -d tinyurl
```

Useful checks:

```sql
\dt
\d links
\d idempotency_keys
\d redirect_events
```

## Run `linkd` With Postgres

By default, `linkd` uses in-memory storage. To run it against local Postgres, set the storage backend and database URL before starting the service:

```powershell
$env:TINYURL_STORAGE='postgres'
$env:TINYURL_DATABASE_URL='postgres://tinyurl:tinyurl@localhost:5433/tinyurl?sslmode=disable'
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/linkd
```

In another PowerShell window, create a link:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/v1/links `
  -ContentType "application/json" `
  -Headers @{"Idempotency-Key" = "local-retry-1"} `
  -Body '{"destination":"https://example.com","ownerId":"owner-1"}'
```

Verify the row exists:

```powershell
docker compose -f deploy/local/compose.yaml exec postgres `
  psql -U tinyurl -d tinyurl -c "select code, destination_url, owner_id, status, version from links;"
```

## Reset Local Data

Stop Postgres:

```powershell
docker compose -f deploy/local/compose.yaml down
```

Delete local database data:

```powershell
docker compose -f deploy/local/compose.yaml down -v
```

Then start Postgres and run migrations again.

## Local Connection String

When the Go Postgres adapters are added, they should use:

```text
postgres://tinyurl:tinyurl@localhost:5433/tinyurl?sslmode=disable
```

Do not commit real production credentials. Local credentials here are intentionally disposable.
