# Go Mental Model

## Big Picture

Go is an imperative, statically typed language. It is not primarily a functional language.

Compared with JavaScript:

| JavaScript                           | Go                                                 |
| ------------------------------------ | -------------------------------------------------- |
| Objects can have changing shapes     | Structs have fixed fields and types                |
| Classes combine data and methods     | Structs hold data; methods are declared separately |
| Most values are dynamically typed    | Every value has a compile-time type                |
| Interfaces are usually informal      | Interfaces are compiler-checked behavior contracts |
| Objects are generally reference-like | Struct values are copied unless pointers are used  |
| Exceptions may be thrown             | Errors are ordinary returned values                |

The most useful summary is:

```text
struct    = fixed-shape data
method    = function associated with a type
interface = behavior required from a value
pointer   = reference to an existing value
package   = namespace and visibility boundary
```

## Packages and Visibility

Every Go file begins with its package:

```go
package domain
```

Names beginning with uppercase letters are exported from the package:

```go
type Link struct{}       // exported
func NewLink() Link      // exported
```

Names beginning with lowercase letters are private to the package:

```go
func validateTime()      // package-private
var internalState string // package-private
```

This is why the TinyURL aggregate exposes `Status()` but stores `status` privately.

## Structs

A struct groups fields with fixed types:

```go
type Link struct {
	code   string
	status LinkStatus
}
```

Create a value:

```go
link := Link{
	code:   "abc123",
	status: Active,
}
```

Unlike a JavaScript object, arbitrary fields cannot be added later.

## Zero Values

Every Go type has an automatic zero value:

| Type     | Zero value                          |
| -------- | ----------------------------------- |
| `string` | `""`                                |
| `bool`   | `false`                             |
| Integer  | `0`                                 |
| Pointer  | `nil`                               |
| Struct   | All fields set to their zero values |

Good Go APIs either make the zero value useful or make it easy to detect as invalid.

Examples from TinyURL:

- `Unknown` is the zero `LinkStatus` and is invalid.
- `DestinationURL{}` is detectable using `IsZero()`.
- `time.Time{}` is detectable using `IsZero()`.

## Functions and Multiple Returns

Functions declare parameter and return types:

```go
func Add(a int, b int) int {
	return a + b
}
```

Go commonly returns a result and an error:

```go
func NewDestinationURL(raw string) (DestinationURL, error)
```

Usage:

```go
destination, err := NewDestinationURL(raw)
if err != nil {
	return err
}
```

Errors are values, not exceptions.

## Methods and Receivers

A method is a function attached to a type through a receiver:

```go
func (l Link) Code() string {
	return l.code
}
```

Read it as:

> Define a method named `Code` on `Link`; inside the method, call the receiver `l`.

The receiver is similar to JavaScript's `this`, but its value-or-pointer form is explicit.

## Values and Copies

Assigning a struct value copies it:

```go
first := Link{status: Active}
second := first

second.status = Disabled
// first remains Active
```

A copied struct may still contain fields such as pointers, slices, or maps that refer to shared underlying data. Copying is shallow.

## Pointers

A pointer stores the address of another value:

```go
link := Link{}
pointer := &link
```

Symbols:

```text
*Link     type: pointer to Link
&link     expression: address of link
*pointer  expression: value at pointer's address
nil       pointer points to no value
```

Example:

```go
var pointer *Link
pointer = &link
copyOfLink := *pointer
```

`&Link` is not a type. `&` needs a value expression:

```go
pointer := &Link{} // valid: address of a newly created Link value
```

## Value and Pointer Receivers

Value receiver:

```go
func (l Link) Status() LinkStatus {
	return l.status
}
```

The receiver is copied. Use this for small, read-only, value-like behavior.

Pointer receiver:

```go
func (l *Link) Disable() {
	l.status = Disabled
}
```

The method accesses the original value. Use this when mutation must persist.

The TinyURL lifecycle mutations require pointer receivers because changing a copied `Link` would have no lasting effect.

## Interfaces

An interface describes behavior:

```go
type Clock interface {
	Now() time.Time
}
```

It specifies:

- Method name: `Now`
- Parameters: none
- Returns: one `time.Time`

It does not specify:

- Receiver type
- Fields
- Constructors
- Internal implementation
- Additional methods

Any appropriate concrete type with a matching method satisfies the interface.

## Why Interfaces Are Implicit

Go uses structural typing for interfaces:

> A type satisfies an interface because it has the required methods, not because it declared a relationship.

This allows the consumer to define a narrow contract:

```go
type Clock interface {
	Now() time.Time
}
```

Existing or third-party types can satisfy it without being modified.

The trade-off is weaker discoverability and the possibility of accidental matches. Important implementations often include an explicit compiler assertion:

```go
var _ Clock = (*fakeClock)(nil)
```

## Errors

Create a sentinel error:

```go
var ErrLinkNotFound = errors.New("link not found")
```

Return it:

```go
return ErrLinkNotFound
```

Check it:

```go
if errors.Is(err, ErrLinkNotFound) {
	// handle absence
}
```

`errors.Is` continues to work when the error is wrapped with additional context.

## Tests

Go test files end with `_test.go`:

```go
func TestLinkStatus(t *testing.T) {
	got := Active.CanRedirect()
	if !got {
		t.Fatal("expected active status to redirect")
	}
}
```

`*testing.T` is a pointer supplied by the test framework. Test methods update the actual running test state.

## Context: Why `With...` Creates Something Different

`context.Context` represents the lifetime of an operation, usually a request.

Contexts are immutable. A `With...` function does not modify its input context. It creates a new child context containing an additional capability:

```go
root := context.Background()
requestCtx, cancelRequest := context.WithCancel(root)
databaseCtx, cancelDatabase := context.WithTimeout(requestCtx, time.Second)
```

This creates a tree:

```text
root
└── requestCtx: can be cancelled explicitly
    └── databaseCtx: automatically cancels after one second
```

The contexts differ because they represent different operation scopes:

- `root` has no request-specific cancellation.
- `requestCtx` represents the whole request.
- `databaseCtx` represents one bounded database operation inside that request.

Cancelling a parent cancels all descendants. Cancelling a child does not cancel its parent.

This is similar to JavaScript's `AbortController`:

```js
const controller = new AbortController();
fetch(url, { signal: controller.signal });
controller.abort();
```

In Go, the cancellation signal is carried through `ctx`:

```go
ctx, cancel := context.WithCancel(context.Background())
repository.FindByCode(ctx, code)
cancel()
```

Use context for cancellation, deadlines, and small request-scoped metadata. Do not use it instead of ordinary function parameters.

## `defer`: Cleanup When the Current Function Returns

`defer` schedules a function call to run when the surrounding function finishes:

```go
func doWork() {
	resource := acquireResource()
	defer resource.Close()

	use(resource)
}
```

`resource.Close()` runs when `doWork` returns, including when it returns early because of an error.

This makes cleanup reliable:

```go
func load(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := firstStep(ctx); err != nil {
		return err
	}

	return secondStep(ctx)
}
```

Without `defer`, every return path would need to remember to call `cancel()`.

For contexts, `defer cancel()` releases timer and cancellation resources as soon as the function is done, even if the timeout has not fired.

Deferred calls run in last-in, first-out order:

```go
defer fmt.Println("first")
defer fmt.Println("second")

// Prints:
// second
// first
```

Use `defer` for function-scoped cleanup such as cancellation, closing files, unlocking mutexes, and ending tracing spans.

Avoid deferring cleanup inside a long-running loop when resources should be released during each iteration; deferred calls wait until the surrounding function returns.
