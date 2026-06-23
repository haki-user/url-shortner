# Go Question Log

This document records questions that arose while building TinyURL. Add new questions when a concept feels unclear or surprising.

## Why Is It `*Link`, Not `&Link`, in a Type?

`*Link` is the type “pointer to Link.”

`&link` is an expression meaning “address of this link value.”

```go
var pointer *Link
link := Link{}
pointer = &link
```

## Are Pointer Receivers Always Faster?

No. They avoid copying the struct, but introduce indirection, possible `nil` values, shared mutation, and sometimes heap allocation. Use them primarily for mutation, identity, large structs, or consistency, not as an automatic optimization.

## Why Are Go Interfaces Implemented Implicitly?

Implicit implementation lets consumers define the narrow behavior they need without modifying or coupling concrete types to those interfaces. It supports third-party compatibility and dependency inversion.

The trade-off is reduced discoverability and possible accidental matches. Use compile-time assertions when explicit intent matters:

```go
var _ LinkRepository = (*PostgresRepository)(nil)
```

## Why Does a Pointer Receiver Affect Interface Satisfaction?

If a method is defined on `*T`, only `*T` is guaranteed to provide that method in every context. A plain `T` may be a non-addressable temporary value, so Go does not treat it as implementing that interface.

| Receiver        | `T` satisfies? | `*T` satisfies? |
| --------------- | -------------: | --------------: |
| `func (T) M()`  |            Yes |             Yes |
| `func (*T) M()` |             No |             Yes |

## Why Does the `Clock` Interface Not Specify a Receiver?

An interface describes caller-visible behavior:

```go
type Clock interface {
	Now() time.Time
}
```

The implementing type decides whether `Now` uses a value or pointer receiver. That choice determines which concrete type satisfies the interface.

## Why Use a Pointer for a Fake Clock?

It is unnecessary for a fixed immutable fake. A value receiver works well.

A pointer becomes useful when tests need to mutate shared fake-clock state:

```go
func (f *fakeClock) Advance(d time.Duration)
```

## Why Did Link Mutations Initially Not Work?

The methods used value receivers:

```go
func (l Link) Disable()
```

This modified a copy. Changing to a pointer receiver made the mutation persist:

```go
func (l *Link) Disable()
```

## Why Copy an Expiration Pointer Before Returning It?

Returning the internal pointer would allow external callers to mutate the aggregate without using its methods. Returning a copy protects the invariant:

```go
copied := *l.expiresAt
return &copied
```

## Why Use an Extra Variable Before Calling `Equal`?

`ExpiresAt()` returns `*time.Time` and may return `nil`. Check it before dereferencing or calling a method:

```go
actual := link.ExpiresAt()
if actual == nil {
	t.Fatal("expected expiration")
}

if !actual.Equal(expected) {
	// fail
}
```

Calling `link.ExpiresAt().Equal(expected)` could panic when no expiration exists.

## Why Does `!now.Before(expiresAt)` Mean Expired?

`Before` is true only when `now` is earlier.

Negating it includes equality and later times:

```text
not before = equal or after
```

That implements inclusive expiration:

```text
now >= expiresAt
```

## What Is the Purpose of `context.With...`?

Contexts are immutable operation-lifetime objects. A `With...` function derives a different child context from a parent:

```go
root := context.Background()
requestCtx, cancel := context.WithCancel(root)
```

`root` and `requestCtx` differ because `requestCtx` carries a cancellation signal while `root` does not.

Different child contexts allow different scopes:

```text
request context
├── database context with a one-second timeout
└── event-publication context with a shorter timeout
```

Cancelling the request context cancels both children. Cancelling one child does not cancel the request or its sibling.

This resembles JavaScript's `AbortController` and `AbortSignal`.

## Why Use a Derived Context When Testing Forwarding?

A derived context is a distinct instance. A fake dependency can capture the received context, and the test can verify the application forwarded the supplied context instead of replacing it with `context.Background()`.

The cancellation behavior is not the main point of that particular test; distinct identity makes incorrect replacement detectable.

## Why Call `defer cancel()`?

`defer` schedules `cancel()` to run when the current function returns, including early error returns:

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()
```

Calling `cancel()` releases context timer and cancellation resources promptly. The timeout may eventually cancel itself, but waiting for it unnecessarily retains resources.

Using `defer` keeps cleanup next to acquisition and prevents missing cleanup on one of several return paths.

Do not blindly use `defer` inside long-running loops, because cleanup waits until the surrounding function returns rather than the current iteration ending.

## Why Check Context Cancellation with `ctx.Err()`?

```go
if err := ctx.Err(); err != nil {
	return err
}
```

Checking `ctx.Err()` at the entry point of repository methods verifies if the operation's context is already canceled (`context.Canceled`) or has timed out (`context.DeadlineExceeded`).

This check prevents executing unnecessary work (such as acquiring database/mutex locks or performing operations on internal maps) when the request has already been aborted or timed out by the caller.

## What Is the Difference Between Read Locks and Write Locks?

`sync.RWMutex` provides two levels of locking for access control:

- **Read Lock (`RLock` / `RUnlock`)**:
  - **Shared access**: Multiple goroutines can hold a read lock simultaneously.
  - Used when reading data without mutating it (e.g. `FindByCode`).
  - Permits any number of concurrent readers to run without blocking each other, but blocks any writer from modifying the data.
- **Write Lock (`Lock` / `Unlock`)**:
  - **Exclusive access**: Only one goroutine can hold a write lock at any time.
  - Used when mutating data (e.g. `Insert`, `Update`).
  - Blocks all other readers and writers to prevent race conditions or data corruption.

## Why Assign Test Setup Values to Local Variables?

Instead of inline literal assignments:

```go
clock := integrationClockFake{now: time.Date(2026, 6, 14, 22, 10, 0, 0, time.UTC)}
```

Using a local variable like `now := time.Date(...)` is preferred because:

- It avoids repeating complex structure declarations across multiple assertion calls.
- It guarantees that the value injected into the unit/integration test matches _exactly_ the expected value used in later assertions, reducing copy-paste errors.

## Why Assert Individual Struct Fields Before Structural Equality?

Asserting individual fields first (e.g. via an `assertIntegrationLink` helper) before checking structural equality (`stored != created`):

- **Better Error Diagnostic**: If structural equality fails, the test runner merely shows `expected stored link to equal created link`, which does not explain _which_ fields are mismatched. Checking fields individually provides precise, debuggable error messages (e.g., `expected code "abc123", got "xyz"`).
- **Robust Verification**: It ensures each field is validated against expected values before ensuring the structs are completely identical.

## How Does `TestRepositoryInsertConcurrentUniqueLinks` Work?

This test validates that the in-memory repository is concurrency-safe and correctly handles concurrent writes to the underlying map using mutexes:

1. **`sync.WaitGroup`**: Orchestrates starting and waiting for multiple concurrent goroutines (100 in total).
2. **Goroutines (`go func()`)**: Initiates 100 parallel execution flows. Each flow creates a unique link (e.g., `code-000`, `code-001`, etc.) and concurrent-writes it to the repository using `repository.Insert`.
3. **Buffered Error Channel (`chan error`)**: Collects error responses from the concurrent inserts. Since channels in Go are thread-safe, multiple goroutines can safely send values to the channel concurrently.
4. **Mutex Synchronization (`r.mu.Lock()`)**: Internally, `Insert` locks `r.mu`. If `Insert` did not use a mutex, Go would panic with `fatal error: concurrent map writes`. By using `Lock` and `Unlock`, only one goroutine modifies the map at a time, making the adapter thread-safe.

## What Are Struct Tags (e.g. `json:"destination"`) in Go?

Struct tags are annotations attached to fields of a struct:

```go
type createLinkHTTPRequest struct {
	Destination string `json:"destination"`
}
```

- **Purpose**: They provide metadata about the fields that can be read via reflection at runtime.
- **JSON Mapping**: The Go standard library's `encoding/json` package uses these tags to map JSON key names (usually in camelCase or snake_case) to the Go struct's fields (which must be capitalized/exported to be visible to the JSON encoder/decoder).

## Why Is `http.ResponseWriter` Not Passed as a Pointer?

In handlers, you write:

```go
func (h Handler) writeRedirectErr(w http.ResponseWriter, err error)
```

Instead of:

```go
func (h Handler) writeRedirectErr(w *http.ResponseWriter, err error)
```

- **Reason**: `http.ResponseWriter` is an **interface** type. In Go, interfaces are lightweight header structures containing pointers to the concrete implementation and its data. Passing an interface by value already functions as a reference to the same underlying connection/writer, so passing a pointer to an interface (`*http.ResponseWriter`) is unnecessary and considered an anti-pattern.

## Why Set the `Content-Type` Header to `application/json`?

```go
w.Header().Set("Content-Type", "application/json")
```

Setting headers provides crucial metadata to the client about the HTTP response payload. Declaring `Content-Type: application/json` explicitly instructs the receiving client (e.g. browsers, frontend frameworks, or curl) to parse the response body as JSON.

## Why Use `w.WriteHeader(statusCode)` Instead of `Header().Set(...)`?

- **`WriteHeader`**: Sets the HTTP response status code (e.g., `201 Created` or `400 Bad Request`) and sends the status line and headers to the client.
- **`Header().Set`**: Modifies the key-value HTTP headers metadata.
- **Ordering**: Status code and headers are distinct parts of the HTTP protocol. In Go, calling `WriteHeader` commits the headers. Any headers set after calling `WriteHeader` will not be sent to the client.

## Why Omit the Receiver Name in a Method Definition (e.g. `(SystemClock)`)?

In Go, you can define a method with just the type name in the receiver block:

```go
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
```

- **No State Accessed**: Since `SystemClock` is an empty struct and the method does not read or write any fields of the struct, the receiver variable is not needed in the method body.
- **Avoid Unused Variable Warnings**: If you declare a receiver name (like `s`) but do not use it in the body, strict compilers or linters may flag it as an unused variable. Omitting the name completely prevents this.
- **Interface Compliance**: Even without naming the variable, the method is bound to the `SystemClock` type, allowing it to satisfy interface contracts like `ports.Clock`.

## Why Assert Interfaces with `SystemClock{}` vs. `(*StaticGenerator)(nil)`?

In Go, you will see two patterns for verifying that a struct implements an interface at compile time:

```go
var _ ports.Clock = SystemClock{}                 // Pattern A (Value type)
var _ ports.CodeGenerator = (*StaticGenerator)(nil) // Pattern B (Pointer type)
```

The difference comes down to **value receivers** vs. **pointer receivers**:

### 1. Value Receivers (`T` implements the interface)

If the struct methods are defined using a value receiver:

```go
func (SystemClock) Now() time.Time
```

Both the value type (`SystemClock`) and pointer type (`*SystemClock`) implement the interface. Using `SystemClock{}` is simple because it creates a zero-value instance of the struct at compile time.

### 2. Pointer Receivers (`*T` implements the interface)

If the struct methods are defined using a pointer receiver:

```go
func (g *StaticGenerator) Generate(...)
```

Only the pointer type (`*StaticGenerator`) satisfies the interface. Writing `var _ ports.CodeGenerator = StaticGenerator{}` will fail to compile.

To assert compilation for the pointer type without instantiating/allocating memory on the heap (such as `&StaticGenerator{}`), we cast `nil` to a typed pointer: `(*StaticGenerator)(nil)`. This is a compile-time check that incurs zero runtime allocation cost.

## Why Use Direct Instantiation (`SystemClock{}`) Instead of a Constructor (`NewSystemClock()`)?

In Go, we initialize dependencies in different ways:
```go
clock := system.SystemClock{}
generator := codegen.NewStaticGenerator()
repository := memory.NewRepository()
```

The choice of whether to write and use a constructor function (like `New...`) depends on whether the struct requires **internal state initialization**:

### 1. Zero-Value is Ready to Use (e.g. `SystemClock`)
`SystemClock` has no fields (it is an empty struct). Therefore, its zero-value is already fully initialized and ready to use. Writing a constructor function like `NewSystemClock()` is redundant because it would merely return `SystemClock{}` without adding any configuration or allocation logic.

### 2. Initialization is Required (e.g. `Repository`, `StaticGenerator`)
* **`memory.Repository`**: Contains an internal map (`links map[string]domain.Link`). If you instantiate it directly as `memory.Repository{}`, the internal map will be `nil`. Any attempt to write to a `nil` map causes a runtime panic. The constructor `NewRepository()` is required to initialize the map via `make(map[string]domain.Link)`.
* **`codegen.StaticGenerator`**: Holds a mutex and counter variables. Defining a constructor function like `NewStaticGenerator()` ensures we return a pointer (`*StaticGenerator`) and sets up a clean API that manages state setup.

## Why Use Base62 Instead of Base64 for URL Shortening?

* **Character Set Comparison**:
  * **Base62**: Uses 62 characters: `0-9`, `a-z`, and `A-Z`.
  * **Base64**: Uses 64 characters: `0-9`, `a-z`, `A-Z`, plus `+` (plus) and `/` (slash).
* **URL Compatibility**:
  * The additional characters in Base64 (`+` and `/`) have special meanings in URL paths and query strings (`/` represents directories/paths, and `+` represents space encoding).
  * If a shortened URL code contained `+` or `/`, the URL would need URL-encoding (e.g. converting `+` to `%2B`), or it could conflict with backend router paths.
  * Base62 uses only alphanumeric characters, making it completely **URL-safe** without requiring any escape sequences.

## How Does Base62 Encoding Work?

Base62 encoding converts a decimal number (like a database ID or counter) into a base-62 alphanumeric string. It works through repeated division and modulo arithmetic:

1. **Calculate the Remainder**: Get the remainder of `number % 62`. The result will be between `0` and `61`.
2. **Character Lookup**: Map the remainder value to a character in the Base62 alphabet (e.g., `0` -> `'0'`, `10` -> `'a'`, `36` -> `'A'`).
3. **Divide**: Divide the number by 62 (`number = number / 62`).
4. **Loop**: Repeat steps 1 to 3 until the number becomes `0`.
5. **Reverse**: Because the algorithm processes the least significant digit (rightmost) first, the final slice of characters must be reversed to place the most significant digit first.

## Why Use Variadic Parameters (`...ports.IdempotencyStore`) in the Constructor?

In Go, we write:
```go
func NewCreateGeneratedLink(
	repository ports.LinkRepository,
	generator ports.CodeGenerator,
	clock ports.Clock,
	idempotencyStores ...ports.IdempotencyStore,
)
```

And check:
```go
var idempotencyStore ports.IdempotencyStore
if len(idempotencyStores) > 0 {
	idempotencyStore = idempotencyStores[0]
}
```

* **Optional Parameters**: Go does not support method overloading or default parameter values. Using a variadic (`...`) slice parameter makes the `IdempotencyStore` optional.
* **Backward Compatibility**: Any existing tests or setups that call `NewCreateGeneratedLink(repo, gen, clock)` continue compiling without needing code modifications.
* **Length Check**: A variadic parameter translates to a slice inside the function. Checking `len(idempotencyStores) > 0` verifies if the caller provided the parameter, permitting us to assign the first element safely.

## How Does Idempotency Work in Link Generation?

Idempotency guarantees that making the same request multiple times has the exact same outcome without causing duplicate work or duplicate database records.

When `CreateGeneratedLink` executes:

### 1. The Pre-Check (GET)
If the request includes an `IdempotencyKey`, the use-case checks the store:
```go
link, err := c.idempotencyStore.Get(ctx, request.OwnerID, request.IdempotencyKey)
```
* **If Found**: The request was already processed successfully. The use-case immediately returns the previously generated `Link`, skipping code generation and insertion.
* **If Not Found**: This is a fresh request. The use-case proceeds to generate a new short code and builds the link.

### 2. The Post-Save (SAVE)
After generating and inserting the link successfully, the use-case saves the result:
```go
c.idempotencyStore.Save(ctx, request.OwnerID, request.IdempotencyKey, link)
```
If a client retries due to network failure, the next attempt hits the pre-check and returns the saved link instead of creating duplicate short links.

## Why Check `!errors.Is(err, ports.ErrIdempotencyKeyNotFound)` When Getting a Key?

In `CreateGeneratedLink.Execute`, we check:
```go
if !errors.Is(err, ports.ErrIdempotencyKeyNotFound) {
	return domain.Link{}, err
}
```

* **The Negation (`!`)**: Note the exclamation mark. This translates to: *"If the error is **NOT** `ErrIdempotencyKeyNotFound`, return the error."*
* **Expected vs. Unexpected Errors**:
  * **Expected**: `ErrIdempotencyKeyNotFound` is completely normal. It means the client is making this request for the first time. The check evaluates to `false`, we **do not** return the error, and we proceed with generating a new link.
  * **Unexpected**: Any other error (like a database connection failure, network timeout, or Redis crash) is unexpected. In those cases, the condition evaluates to `true`, and we immediately return the error to the caller instead of continuing blindly.

## What is the Difference Between `r` (`*http.Request`) and `request` in Handlers?

In a typical Go handler, you see both:
* `r *http.Request` (as a method parameter)
* `request createLinkHTTPRequest` (as a local variable)

An easy way to understand the difference is:

### 1. `r` (`*http.Request`) — The Envelope
`r` represents the entire HTTP request received from the network. It contains all HTTP metadata, such as:
* HTTP headers (e.g. `r.Header.Get("Idempotency-Key")`)
* HTTP method (e.g. `r.Method == http.MethodPost`)
* Request path and query params (e.g. `r.URL.Path`)
* The raw request body stream (`r.Body`)

### 2. `request` (e.g. `createLinkHTTPRequest`) — The Letter Inside
`request` is a custom struct we define to hold the unmarshaled (deserialized) data specifically parsed from the request body's JSON payload:
```go
var request createLinkHTTPRequest
json.NewDecoder(r.Body).Decode(&request)
```
It only contains the keys decoded from the JSON body (e.g., `Destination`, `OwnerID`). It **does not** know about headers, URLs, cookies, or connection metadata because they are not part of the body payload.

* **Guidance**: Use `r.Header.Get("...")` when you need HTTP transport information (like custom headers, auth tokens, client IPs). Use fields on `request` when accessing values sent in the JSON body.

## Topics to Revisit

- Slices, maps, and their reference-like behavior.
- `context.Context` and cancellation.
- Goroutines and channels.
- Error wrapping.
- Generic types.
- Memory allocation and escape analysis.





