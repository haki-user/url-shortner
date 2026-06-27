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

- **`memory.Repository`**: Contains an internal map (`links map[string]domain.Link`). If you instantiate it directly as `memory.Repository{}`, the internal map will be `nil`. Any attempt to write to a `nil` map causes a runtime panic. The constructor `NewRepository()` is required to initialize the map via `make(map[string]domain.Link)`.
- **`codegen.StaticGenerator`**: Holds a mutex and counter variables. Defining a constructor function like `NewStaticGenerator()` ensures we return a pointer (`*StaticGenerator`) and sets up a clean API that manages state setup.

## Why Use Base62 Instead of Base64 for URL Shortening?

- **Character Set Comparison**:
  - **Base62**: Uses 62 characters: `0-9`, `a-z`, and `A-Z`.
  - **Base64**: Uses 64 characters: `0-9`, `a-z`, `A-Z`, plus `+` (plus) and `/` (slash).
- **URL Compatibility**:
  - The additional characters in Base64 (`+` and `/`) have special meanings in URL paths and query strings (`/` represents directories/paths, and `+` represents space encoding).
  - If a shortened URL code contained `+` or `/`, the URL would need URL-encoding (e.g. converting `+` to `%2B`), or it could conflict with backend router paths.
  - Base62 uses only alphanumeric characters, making it completely **URL-safe** without requiring any escape sequences.

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

- **Optional Parameters**: Go does not support method overloading or default parameter values. Using a variadic (`...`) slice parameter makes the `IdempotencyStore` optional.
- **Backward Compatibility**: Any existing tests or setups that call `NewCreateGeneratedLink(repo, gen, clock)` continue compiling without needing code modifications.
- **Length Check**: A variadic parameter translates to a slice inside the function. Checking `len(idempotencyStores) > 0` verifies if the caller provided the parameter, permitting us to assign the first element safely.

## How Does Idempotency Work in Link Generation?

Idempotency guarantees that making the same request multiple times has the exact same outcome without causing duplicate work or duplicate database records.

When `CreateGeneratedLink` executes:

### 1. The Pre-Check (GET)

If the request includes an `IdempotencyKey`, the use-case checks the store:

```go
link, err := c.idempotencyStore.Get(ctx, request.OwnerID, request.IdempotencyKey)
```

- **If Found**: The request was already processed successfully. The use-case immediately returns the previously generated `Link`, skipping code generation and insertion.
- **If Not Found**: This is a fresh request. The use-case proceeds to generate a new short code and builds the link.

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

- **The Negation (`!`)**: Note the exclamation mark. This translates to: _"If the error is **NOT** `ErrIdempotencyKeyNotFound`, return the error."_
- **Expected vs. Unexpected Errors**:
  - **Expected**: `ErrIdempotencyKeyNotFound` is completely normal. It means the client is making this request for the first time. The check evaluates to `false`, we **do not** return the error, and we proceed with generating a new link.
  - **Unexpected**: Any other error (like a database connection failure, network timeout, or Redis crash) is unexpected. In those cases, the condition evaluates to `true`, and we immediately return the error to the caller instead of continuing blindly.

## What is the Difference Between `r` (`*http.Request`) and `request` in Handlers?

In a typical Go handler, you see both:

- `r *http.Request` (as a method parameter)
- `request createLinkHTTPRequest` (as a local variable)

An easy way to understand the difference is:

### 1. `r` (`*http.Request`) — The Envelope

`r` represents the entire HTTP request received from the network. It contains all HTTP metadata, such as:

- HTTP headers (e.g. `r.Header.Get("Idempotency-Key")`)
- HTTP method (e.g. `r.Method == http.MethodPost`)
- Request path and query params (e.g. `r.URL.Path`)
- The raw request body stream (`r.Body`)

### 2. `request` (e.g. `createLinkHTTPRequest`) — The Letter Inside

`request` is a custom struct we define to hold the unmarshaled (deserialized) data specifically parsed from the request body's JSON payload:

```go
var request createLinkHTTPRequest
json.NewDecoder(r.Body).Decode(&request)
```

It only contains the keys decoded from the JSON body (e.g., `Destination`, `OwnerID`). It **does not** know about headers, URLs, cookies, or connection metadata because they are not part of the body payload.

- **Guidance**: Use `r.Header.Get("...")` when you need HTTP transport information (like custom headers, auth tokens, client IPs). Use fields on `request` when accessing values sent in the JSON body.

## Why Must We Re-Assign Slices When Using `append`?

In Go, we write:

```go
r.events = append(r.events, event)
```

Instead of just:

```go
append(r.events, event)
```

This is because of how Go handles **slices** under the hood:

### 1. Slice Header Structure

A slice in Go is not a dynamic array itself. It is a tiny, fixed-size header struct containing:

- A pointer to the underlying array.
- The length of the slice (`len`).
- The capacity of the slice (`cap`).

### 2. How `append` Behaves

The `append` function takes the slice header by value and returns a **new** slice header:

- **If there is capacity**: It writes the new element to the next slot in the underlying array and returns a new slice header with `len` incremented by 1.
- **If capacity is exceeded**: It allocates a new, larger array, copies all existing elements over, writes the new element, and returns a new slice header with a updated pointer, `len`, and `cap`.

### 3. Why Re-Assignment is Required

Because `append` always returns a new slice header (and might allocate a completely new backing array), you must assign that returned header back to your variable (e.g. `r.events = ...`). If you call `append` without assigning it back, the original `r.events` variable keeps its old slice header (pointing to the old length and capacity), and you will never see the newly added element.

### Comparison to C++ (`std::vector`)

In C++, a vector manages its internal state in-place:

```cpp
std::vector<int> vec;
vec.push_back(1); // Modifies the object's internal pointers directly
```

- **C++**: `push_back` is a method on the class that mutates the instance `this` pointer directly. If reallocation occurs, the class instance updates its internal pointers in-place transparently.
- **Go**: Go does not use class methods for `append`. Instead, `append` is a built-in function that takes the slice header **by value** (creating a copy of the pointer, length, and capacity). Since Go functions pass arguments by value, `append` cannot mutate the caller's slice variable directly. It must return a modified copy of the slice header, which you must manually re-assign.

## Why Return a Copy of a Slice Instead of the Internal Slice Directly?

In `Events()`, we write:

```go
events := make([]domain.RedirectEvent, len(r.events))
copy(events, r.events)
return events
```

Instead of:

```go
return r.events
```

We do this for two critical reasons:

### 1. Concurrency Safety (Avoiding Data Races)

If we directly return `r.events`, the caller gets a copy of the slice header pointing to the **same underlying array** that the recorder modifies.
If one goroutine calls `Record(...)` (which appends and modifies the underlying array) while another goroutine reads or loops over the returned slice, they will access the same memory concurrently without synchronization, causing a **data race** (which can corrupt memory or panic).

By allocating a new slice and copying the elements _while holding the mutex lock_, the caller gets a completely isolated backing array. They can safely read it without needing to synchronize locks with future `Record` calls.

### 2. Preventing External Mutation (Encapsulation)

Slices share memory. If we return `r.events` directly, an external caller could modify the elements (e.g., `events[0] = dummyEvent`), which would alter the recorder's private internal state from outside the struct. Returning a copy protects the struct's invariants.

## What is the Functional Options Pattern in Go?

In `handler.go`, you see this type definition:

```go
type HandlerOption func(*Handler)
```

This is the definition for the **Functional Options Pattern**, a design pattern in Go used to construct complex objects with optional settings.

### The Problem in Go

Go does not support:

- Method/function overloading (multiple versions of the same function with different parameters).
- Optional arguments with default values (like Python's `def func(a, b=None)`).

If an object needs optional configurations, the naive options are:

1. Creating multiple constructor functions: `NewHandler()`, `NewHandlerWithClock()`, `NewHandlerWithAnalyticsAndClock()`. (This gets messy quickly).
2. Passing empty/nil pointers to a single massive constructor: `NewHandler(create, redirect, baseURL, nil, nil)`. (This makes the client code ugly).

### The Solution: Functional Options

We define a function signature that takes a pointer to our struct:

```go
type HandlerOption func(*Handler)
```

Then we define builder functions that return this option type:

```go
func WithClock(c ports.Clock) HandlerOption {
	return func(h *Handler) {
		h.clock = c
	}
}

func WithAnalytics(rec ports.RedirectEventRecorder) HandlerOption {
	return func(h *Handler) {
		h.analyticsRecorder = rec
	}
}
```

In the constructor, we accept a variadic parameter:

```go
func NewHandler(create, redirect, baseURL, options ...HandlerOption) *Handler {
	h := &Handler{
		createGeneratedLink: create,
		redirectLink:        redirect,
		baseURL:             baseURL,
		clock:               system.SystemClock{}, // default value
	}

	for _, opt := range options {
		opt(h) // Apply each option function
	}

	return h
}
```

### Why Use It?

- **API Flexibility**: Callers only pass the configurations they care about:
  ```go
  h := NewHandler(create, redirect, url, WithClock(myClock), WithAnalytics(myRecorder))
  ```
- **Extensibility & Backwards Compatibility**: If we add a new config setting to `Handler` in the future, we just add a new `WithXYZ` function. Existing code will continue to compile and run without modifications.

### What is the `for _, option := range options` Loop Doing?

This loop evaluates the optional configuration functions:

1. **`options` is a Slice**: Since the constructor parameter is variadic (`options ...HandlerOption`), Go groups all arguments passed after the base parameters into a slice of functions: `[]HandlerOption` (i.e. `[]func(*Handler)`).
2. **Sequential Invocation**: The `for` loop iterates over each configuration function in the slice.
3. **`option(&handler)`**: For each configuration function, we invoke it, passing a pointer to our newly created `handler` instance (`&handler`). This function then modifies the handler in place.
   For example, if the caller passed `WithAnalytics(recorder, clock)`, the function executes:
   ```go
   func(h *Handler) {
       h.analyticsRecorder = recorder
       h.clock = clock
   }
   ```
   This modifies `handler.analyticsRecorder` and `handler.clock` directly. Once the loop ends, the struct has all configurations applied.

## What is `http.ResponseWriter` and Why Do We Use It?

In Go's `net/http` package, handler methods have this signature:

```go
func ServeHTTP(w http.ResponseWriter, r *http.Request)
```

`w` is the **Response Writer**, which represents the network output stream you use to write data back to the client.

### 1. It is an Interface

`http.ResponseWriter` is defined as a simple interface with three methods:

```go
type ResponseWriter interface {
	Header() Header
	WriteHeader(statusCode int)
	Write([]byte) (int, error)
}
```

- **`Header()`**: Returns the HTTP headers map. You call this to set metadata headers (e.g. `w.Header().Set("Content-Type", "application/json")`).
- **`WriteHeader(statusCode)`**: Sends the HTTP status line and commits headers.
- **`Write([]byte)`**: Writes the response body bytes to the connection stream. It implements Go's standard `io.Writer` interface, meaning it integrates with standard writers (like `json.NewEncoder(w).Encode(...)`).

### 2. Contrast with `r *http.Request`

- **`r` (Request)**: A struct pointer. You **read** data from it (Incoming data from the client, such as URL path, request body, client headers).
- **`w` (ResponseWriter)**: An interface. You **write** data to it (Outgoing data to send back to the client).

## Why Use `http.StatusFound` (302) Instead of a Permanent Redirect (301) for URL Shorteners?

In our redirect handler, we trigger:

```go
http.Redirect(w, r, result.Destination, http.StatusFound)
```

`http.StatusFound` corresponds to HTTP status code **302 Found** (historically "Moved Temporarily"). We choose this over **301 Moved Permanently** for several key reasons:

### 1. Browser Caching and Analytics (Crucial)

- **301 Moved Permanently**: Browsers aggressively cache 301 redirects. The next time a user clicks the same short link, the browser directly loads the destination page from its local cache **without hitting our server**. Consequently, we would fail to record any analytics (UA, referer, IP, click counts) for subsequent visits.
- **302 Found**: Browsers do not cache 302 redirects by default. The browser must request our server every single time, enabling us to reliably track analytics events on every redirect.

### 2. Updatable Destinations

If the owner of a short URL changes the destination (e.g. from `google.com` to `bing.com`):

- With a **301**, returning users who previously visited the link will continue to be redirected to `google.com` by their local browser cache.
- With a **302**, the client requests our server again, immediately receiving the updated destination.

## What is `isCodePath` and How Does It Route Requests?

In `handler.go`, we check:

```go
if r.Method == http.MethodGet && isCodePath(r.URL.Path) {
	h.handleRedirect(w, r)
}
```

The `isCodePath` helper function is a lightweight routing mechanism that checks if the request path represents a short code (e.g. `/abc123`) rather than a nested API path (e.g. `/v1/links`).

### Step-by-Step Logic

```go
func isCodePath(path string) bool {
	// 1. Exclude the root path "/" and empty paths
	if path == "" || path == "/" {
		return false
	}

	// 2. HTTP paths should start with "/"
	if !strings.HasPrefix(path, "/") {
		return false
	}

	// 3. Ensure the path is exactly one segment (contains no other "/")
	return !strings.Contains(strings.TrimPrefix(path, "/"), "/")
}
```

- **Example `path = "/abc123"`**:
  - Stripping `/` leaves `"abc123"`.
  - `"abc123"` does not contain `/`.
  - **Result**: `true` (valid short link code).
- **Example `path = "/v1/links"`**:
  - Stripping `/` leaves `"v1/links"`.
  - `"v1/links"` contains a `/`.
  - **Result**: `false` (this is an API path, not a short code).

This allows us to route short URL redirections directly without requiring a third-party router library.

## How and Where is `WithAnalytics` Called?

In `handler.go`, you define:

```go
func WithAnalytics(
	recorder analyticsports.RedirectEventRecorder,
	clock ports.Clock,
) HandlerOption {
	return func(h *Handler) {
		h.analyticsRecorder = recorder
		h.clock = clock
	}
}
```

You might wonder where this function actually gets called.

### 1. It is called by the Constructor's Consumer

`WithAnalytics` is called by the code that instantiates your HTTP handler. For example, in your unit tests:

```go
handler := NewHandler(
	createGeneratedLink,
	redirectLink,
	baseURL,
	WithAnalytics(analyticsRecorder, clock), // 👈 Called here!
)
```

### 2. Execution Flow

1. **Calling `WithAnalytics(...)`**:
   The caller executes `WithAnalytics(recorder, clock)`. This function immediately returns a closure (an anonymous function) of type `HandlerOption`:
   ```go
   func(h *Handler) {
       h.analyticsRecorder = recorder
       h.clock = clock
   }
   ```
2. **Passing to `NewHandler`**:
   This returned function is passed into `NewHandler` as part of the variadic `options ...HandlerOption` arguments slice.
3. **Execution inside `NewHandler`**:
   Inside the `NewHandler` constructor, we loop over the options and execute each closure, passing a pointer to the handler instance:
   ```go
   for _, option := range options {
       option(&handler) // 👈 This executes the closure returned in step 1!
    }
    ```

## Why Is `http.ResponseWriter` an Interface Instead of a Pointer?

The HTTP server owns a concrete mutable response object. Internally, it creates
an explicit pointer and passes that pointer as an interface:

```go
response := &internalResponse{}

var writer http.ResponseWriter = response
handler.ServeHTTP(writer, request)
```

The interface does not create a pointer automatically. Its dynamic value happens
to be the `*internalResponse` pointer supplied by the server. Passing the
interface to another function copies the interface value, but both copies still
refer to the same response object.

Handlers accept the interface instead of the concrete pointer because they only
need three capabilities:

```go
type ResponseWriter interface {
    Header() http.Header
    WriteHeader(statusCode int)
    Write(body []byte) (int, error)
}
```

This boundary allows production HTTP implementations, test response recorders,
and middleware wrappers to provide different concrete response types. A
`*http.ResponseWriter` would merely add a redundant pointer to the interface;
the concrete pointer is already stored inside the interface value.

No response is returned because the writer streams headers and body bytes to
the server-owned network response. Streaming avoids constructing the entire
response body in memory before sending it.

## What Is the Function Adapter Pattern?

The problem comes first: an API expects a value with a method, but we only have
a plain function with matching parameters.

```text
API expects: handler.ServeHTTP(w, r)
We have:     hello(w, r)
```

A **function adapter** converts the function into the method-based shape
required by the interface. It adds no business behavior; its method only
forwards the call.

```go
type HandlerFunc func(http.ResponseWriter, *http.Request)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    f(w, r)
}
```

`HandlerFunc` is a named function type, not a function declaration. Go permits
methods on locally defined named types, including function types. Since
`HandlerFunc` has the required method, it implicitly satisfies `http.Handler`:

```go
var _ http.Handler = HandlerFunc(nil) // compile-time assertion
```

An ordinary function can now be adapted:

```go
func hello(w http.ResponseWriter, r *http.Request) {
    _, _ = w.Write([]byte("hello"))
}

handler := http.HandlerFunc(hello)
```

The complete flow is:

```text
plain function
    -> convert to HandlerFunc
    -> HandlerFunc provides ServeHTTP
    -> value satisfies http.Handler
```

This lets one API accept both stateful structs and small functions through the
same interface. The `CheckerFunc` readiness design used the same pattern to
adapt `pool.Ping` into a `Checker`.

Do not add this adapter automatically. If the consumer can accept a function
directly, a function field is simpler. Use the adapter when an existing
interface boundary genuinely needs both function and object-style
implementations.

## Why Do Management Reads Return a Version and `ETag`?

TinyURL has two different kinds of reads:

- `GET /{code}` is public redirect traffic. It only needs to decide whether the
  link can redirect and return its destination.
- `GET /v1/links/{code}` is an owner management read. It returns the current
  status, expiration, and version needed to safely edit the link.

The version prevents one update from accidentally overwriting another:

```text
Tab A reads version 1
Tab B reads version 1
Tab A disables the link       -> stored version becomes 2
Tab B updates using version 1 -> rejected as stale
```

HTTP exposes the resource version using an `ETag` response header:

```http
ETag: "1"
```

The client sends that value back when mutating the link:

```http
If-Match: "1"
```

`If-Match` means: apply this change only if the stored resource still has the
version I previously read. The repository enforces the same rule with an
optimistic update:

```sql
update links
set status = $1, version = version + 1
where code = $2 and version = $3;
```

If no row is updated, another request changed the link first. The service
returns a precondition/version-conflict response instead of losing data. This
strategy is called **optimistic concurrency control** because requests proceed
without locking the row while the user thinks, then verify the version at write
time.

An owner header such as `X-Owner-ID` is only a temporary local-development
stand-in. A production service must derive owner identity from verified
authentication or a trusted gateway, not trust a client-supplied owner header.

Ownership is checked in the application use case rather than only in the HTTP
handler. HTTP is one adapter; future gRPC or job adapters must enforce the same
rule. The adapter extracts identity and maps errors to HTTP responses, while the
application layer decides whether that identity may access the link. Ownership
mismatches return `404` to avoid revealing that another owner's code exists.

## How Does the Server Stop?

The server waits for one of two events:

```text
ListenAndServe fails --------> serverErrors channel ----+
                                                        +--> select --> run returns
Ctrl+C or SIGTERM -----------> ctx.Done() channel ------+             |
                                                                      v
                                                             deferred cleanup
```

### Ctrl+C or Application Container Stop

- Ctrl+C normally sends `os.Interrupt`.
- On Linux, `docker stop` normally sends `SIGTERM`, waits for its stop timeout,
  and then uses `SIGKILL` if the process has not exited.
- `signal.NotifyContext` converts `os.Interrupt` or `SIGTERM` into context
  cancellation.
- The `<-ctx.Done()` case becomes ready and starts graceful shutdown.
- `server.Shutdown` stops new connections and waits for active requests.
- If the shutdown timeout expires, `server.Close` forcibly closes connections.
- `run` returns and its deferred database cleanup runs.

### Server Failure

`ListenAndServe` runs in a goroutine and sends its result through
`serverErrors`. For example, it can fail because the address is already in use
or the process cannot open the listening socket. The `serverErrors` case wins,
`run` returns the wrapped error, deferred cleanup runs, and `main` exits with
status 1.

`http.ErrServerClosed` is expected during a requested shutdown, so it is
converted to `nil`.

### Stopping Only Postgres

Stopping the Postgres container does not stop the application. Existing or new
database operations fail while Postgres is unavailable. The application keeps
listening, and the connection pool may reconnect after Postgres returns.
`/healthz` remains `200` because the process is alive, while `/readyz` returns
`503` until Postgres becomes available again.

### When Graceful Shutdown Cannot Run

`SIGKILL`, an out-of-memory kill, power loss, or a machine crash cannot be
handled by Go. Context cancellation and deferred functions do not run. Durable
systems must therefore remain correct after abrupt termination; database
transactions and constraints provide part of that protection.

### `%w` Versus `%v`

- `%w` includes an error and preserves it for `errors.Is` and `errors.As`.
- `%v` includes only its readable text.
- Wrap the primary cause with `%w`. Use `errors.Join` when multiple errors must
  remain programmatically inspectable.

## Topics to Revisit

- Slices, maps, and their reference-like behavior.
- `context.Context` and cancellation.
- Goroutines and channels.
- Error wrapping.
- Generic types.
- Memory allocation and escape analysis.
