# Task 0001: Link Domain Model

## Goal

Implement the core `Link` aggregate and `DestinationURL` value object.

Implementation belongs in:

```text
internal/link/domain/
```

Use only Go's standard library. Do not add persistence, HTTP, JSON, cache, or event-publishing concerns.

## Domain Decisions

- Disabled links can be reactivated.
- Expiration is inclusive: a link cannot redirect when `now >= expiresAt`.
- Deletion is terminal. Deleted links cannot change again.
- Short codes are immutable and must never be reused.
- Destination validation and normalization belong in a separate `DestinationURL` value object.
- Destination URLs may use only `http` or `https`.

## Required Behavior

### Link Aggregate

- Represent active, disabled, and deleted states.
- Support optional expiration.
- Determine whether the link can redirect at a supplied time.
- Disable an active link.
- Reactivate a disabled link.
- Delete a non-deleted link.
- Prevent invalid state transitions.
- Keep internal state protected from uncontrolled mutation.
- Increment the link version after successful mutations.

### DestinationURL Value Object

- Parse and validate a destination URL.
- Accept only `http` and `https`.
- Reject malformed URLs and missing hosts.
- Expose a safe string representation.
- Prevent callers from mutating its internal representation.

### Domain Errors

Define stable domain errors that callers can inspect using `errors.Is`.

## Tests

Write table-driven tests covering:

- Valid and invalid destination URLs.
- Active, disabled, deleted, and expired redirect behavior.
- The exact expiration boundary.
- Valid lifecycle transitions.
- Invalid lifecycle transitions.
- Version changes after successful mutations.
- No version changes after rejected mutations.

## Completion Criteria

- `go test ./internal/link/domain/...` passes.
- `go test -race ./internal/link/domain/...` passes.
- `go vet ./internal/link/domain/...` passes.
- Public identifiers and behavior are understandable without infrastructure context.

## Review Focus

The review will focus on:

- Whether invariants can be bypassed.
- Boundary correctness.
- Error semantics.
- API clarity.
- Test quality.

