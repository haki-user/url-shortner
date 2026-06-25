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
