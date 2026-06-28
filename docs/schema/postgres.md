# Postgres Schema

This document is the implementation reference for TinyURL's first durable storage layer.

The schema supports three data groups:

- shortened link metadata
- create-link idempotency records
- lightweight redirect analytics

## Design Goals

- Preserve existing domain and application boundaries.
- Keep link lookup fast by primary key.
- Make create-link retries safe with a unique idempotency key per owner.
- Store redirect events durably without making analytics part of the link aggregate.
- Use constraints for invariants that the database can enforce cheaply.

## Tables

### `links`

Purpose: source of truth for shortened links.

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `code` | `text` | no | Primary key. Short code used in redirects. |
| `destination_url` | `text` | no | Normalized destination URL. |
| `owner_id` | `text` | no | Owner/account identifier. |
| `status` | `text` | no | Link lifecycle status. |
| `created_at` | `timestamptz` | no | Creation timestamp. |
| `updated_at` | `timestamptz` | no | Last update timestamp. |
| `expires_at` | `timestamptz` | yes | Optional expiration timestamp. |
| `version` | `bigint` | no | Optimistic locking version. |

Constraints:

```sql
primary key (code)
check (status in ('active', 'disabled', 'deleted'))
check (expires_at is null or expires_at > created_at)
check (version >= 1)
```

Indexes:

```sql
create index links_owner_created_idx on links (owner_id, created_at desc);
create index links_expires_at_idx on links (expires_at) where expires_at is not null;
create index links_status_expires_at_idx on links (status, expires_at);
```

Repository behavior:

- `Insert` inserts one row and returns `ErrLinkAlreadyExists` on duplicate `code`.
- `FindByCode` fetches by primary key and returns `ErrLinkNotFound` when missing.
- `Update` uses optimistic locking:

```sql
update links
set destination_url = $1,
    status = $2,
    updated_at = $3,
    expires_at = $4,
    version = $5
where code = $6
  and version = $7;
```

If no row is updated, the adapter should distinguish missing rows from version conflicts when practical:

- missing `code` -> `ErrLinkNotFound`
- existing `code` with different version -> `ErrVersionConflict`

### `idempotency_keys`

Purpose: make create-link retries safe.

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `owner_id` | `text` | no | Owner/account identifier. |
| `key` | `text` | no | Client-provided idempotency key. |
| `code` | `text` | no | Link created for this request. |
| `created_at` | `timestamptz` | no | Record creation timestamp. |

Constraints:

```sql
primary key (owner_id, key)
foreign key (code) references links (code)
```

Indexes:

```sql
create index idempotency_keys_code_idx on idempotency_keys (code);
```

Repository behavior:

- `Get(ownerID, key)` joins or fetches the referenced link and returns `ErrIdempotencyKeyNotFound` when missing.
- `Save(ownerID, key, link)` inserts a new record.
- Re-saving the same `owner_id`, `key`, and `code` is idempotent.
- Saving the same `owner_id` and `key` with a different `code` returns `ErrIdempotencyKeyConflict`.
- The same `key` may be reused by different owners.

Expected insert pattern:

```sql
insert into idempotency_keys (owner_id, key, code, created_at)
values ($1, $2, $3, $4)
on conflict (owner_id, key) do nothing;
```

If no row is inserted, fetch the existing row and compare `code`.

### `redirect_events`

Purpose: lightweight analytics for successful redirects.

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `id` | `bigserial` | no | Primary key. |
| `code` | `text` | no | Link code that redirected. |
| `occurred_at` | `timestamptz` | no | Time of redirect. |
| `user_agent` | `text` | yes | Request user agent. |
| `referer` | `text` | yes | Request referer. |
| `ip` | `text` | yes | Best-effort client IP. |

Constraints:

```sql
primary key (id)
foreign key (code) references links (code)
```

Indexes:

```sql
create index redirect_events_code_occurred_idx on redirect_events (code, occurred_at desc);
create index redirect_events_occurred_at_idx on redirect_events (occurred_at);
```

Repository behavior:

- `Record` appends one event.
- Analytics write failures should not block redirects in the HTTP adapter.
- The first implementation may write synchronously through the recorder port.
- A later implementation can replace this adapter with async buffering or an event stream.

## Full SQL Draft

```sql
create table links (
    code text primary key,
    destination_url text not null,
    owner_id text not null,
    status text not null,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    expires_at timestamptz null,
    version bigint not null,
    check (status in ('active', 'disabled', 'deleted')),
    check (expires_at is null or expires_at > created_at),
    check (version >= 1)
);

create index links_owner_created_idx on links (owner_id, created_at desc);
create index links_expires_at_idx on links (expires_at) where expires_at is not null;
create index links_status_expires_at_idx on links (status, expires_at);

create table idempotency_keys (
    owner_id text not null,
    key text not null,
    code text not null references links (code),
    created_at timestamptz not null,
    primary key (owner_id, key)
);

create index idempotency_keys_code_idx on idempotency_keys (code);

create table redirect_events (
    id bigserial primary key,
    code text not null references links (code),
    occurred_at timestamptz not null,
    user_agent text null,
    referer text null,
    ip text null
);

create index redirect_events_code_occurred_idx on redirect_events (code, occurred_at desc);
create index redirect_events_occurred_at_idx on redirect_events (occurred_at);
```

## Scale Notes

This schema is intentionally simple and relational.

At larger scale:

- `redirect_events` may be partitioned by time.
- analytics writes may move to a queue or event stream.
- foreign keys on analytics events may be removed to reduce write-path coupling.
- link lookup may move to a cache in front of Postgres.
- short-code generation may move to range allocation or a dedicated ID service.
