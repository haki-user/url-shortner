# Pointers, Methods, and Interfaces

## `*Link` Versus `&link`

These are not alternatives:

```go
var pointer *Link
```

`*Link` is a type: pointer-to-Link.

```go
pointer := &link
```

`&link` is an expression: retrieve the address of the existing `link` value.

```go
link := Link{}
var pointer *Link = &link
```

## Why Not Always Use Pointers?

Pointers avoid copying the full struct, but that does not make them universally faster or better.

Pointer costs and risks include:

- Possible `nil` pointer panics.
- Indirection when reading data.
- Potential heap allocation.
- Shared mutable state.
- Harder ownership reasoning.
- More restrictions on interface satisfaction.

A copied string field copies a small descriptor, not all string bytes. Small value-like structs are often inexpensive to copy.

Use pointers when:

- A method mutates the original.
- The struct is large or expensive to copy.
- Shared identity matters.
- The type contains synchronization primitives.
- Consistency with the type's other methods makes pointers clearer.

Use values when:

- The type is small and value-like.
- Copying is safe and expected.
- The method only reads state.

`Link` is a mutable aggregate, so pointer receivers for all methods could be a reasonable consistency choice. Small value objects such as `DestinationURL` are natural value-receiver candidates.

## Receiver Method Sets

Given:

```go
type Clock struct{}
```

### Value Receiver

```go
func (c Clock) Now() time.Time
```

Both method sets include `Now()`:

```text
Clock
*Clock
```

Therefore both can satisfy:

```go
type TimeProvider interface {
	Now() time.Time
}
```

### Pointer Receiver

```go
func (c *Clock) Now() time.Time
```

Only this method set includes `Now()`:

```text
*Clock
```

`Clock` does not satisfy `TimeProvider`; `*Clock` does.

### Summary

| Method declaration      | Value satisfies interface? | Pointer satisfies interface? |
| ----------------------- | -------------------------: | ---------------------------: |
| `func (c Clock) Now()`  |                        Yes |                          Yes |
| `func (c *Clock) Now()` |                         No |                          Yes |

## Why Can a Value Sometimes Call a Pointer Method?

If a value is addressable, Go may automatically take its address:

```go
clock.Now()
```

may be treated like:

```go
(&clock).Now()
```

This convenience applies to direct calls. It does not mean the value type satisfies an interface requiring that pointer method.

Interfaces require the method to be callable reliably in every valid context. Some values, such as map lookup results, are temporary and not addressable:

```go
clocks := map[string]Clock{}
// clocks["main"] is a copied temporary value.
```

## Why the Fake Clock May Use a Pointer

Immutable fake:

```go
type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}
```

Both `fakeClock` and `*fakeClock` satisfy `Clock`.

Mutable fake:

```go
func (f *fakeClock) Advance(d time.Duration) {
	f.now = f.now.Add(d)
}
```

The pointer lets tests mutate and share one clock instance.

If the fake only reads a fixed time, a value receiver is simpler. If tests need to advance it, a pointer receiver is appropriate.

## Optional Values and Pointer Copies

TinyURL stores optional expiration as:

```go
expiresAt *time.Time
```

Meaning:

```text
nil        = no expiration
non-nil    = expiration exists
```

To prevent callers from changing internal state, return a copy:

```go
func (l Link) ExpiresAt() *time.Time {
	if l.expiresAt == nil {
		return nil
	}

	copied := *l.expiresAt
	return &copied
}
```

Step by step:

1. `*l.expiresAt` reads the time stored behind the internal pointer.
2. `copied := ...` creates a separate value.
3. `&copied` returns a pointer to the separate value.
4. The caller can mutate the returned pointer without changing the aggregate.

## Interface Signatures Do Not Include Receivers

```go
type Clock interface {
	Now() time.Time
}
```

The receiver is excluded because the interface describes what callers can do, not how the implementation stores or accesses its state.

These can both provide the same caller-visible behavior:

```go
func (SystemClock) Now() time.Time
func (*FakeClock) Now() time.Time
```

But the receiver determines which concrete type satisfies the interface:

```text
SystemClock and *SystemClock satisfy Clock.
Only *FakeClock satisfies Clock.
```

## Compile-Time Interface Assertions

```go
var _ Clock = (*fakeClock)(nil)
```

Breakdown:

```go
(*fakeClock)(nil)
```

Creates a typed nil value whose type is `*fakeClock`.

```go
var _ Clock = ...
```

Asks the compiler whether that value can be assigned to `Clock`. `_` discards the value because only the type check matters.

This gives explicit intent while preserving Go's implicit interface system.
