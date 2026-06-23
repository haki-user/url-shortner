# Go Cheat Sheet

## Variables

```go
var name string
name = "tinyurl"

count := 1 // short declaration inside functions
```

## Structs

```go
type Link struct {
	code   string
	status LinkStatus
}

link := Link{
	code:   "abc123",
	status: Active,
}
```

Prefer keyed literals:

```go
DestinationURL{value: value}
```

## Functions

```go
func Add(a int, b int) int {
	return a + b
}

func Find() (Link, error) {
	return Link{}, nil
}
```

## Methods

Read-only/value receiver:

```go
func (l Link) Status() LinkStatus
```

Mutating/pointer receiver:

```go
func (l *Link) Disable() error
```

## Pointer Symbols

```text
*Link     pointer-to-Link type
&link     address of link
*pointer  value behind pointer
nil       no pointed-to value
```

```go
link := Link{}
pointer := &link
copy := *pointer
```

## Receiver and Interface Rule

| Method receiver   | Value implements interface? | Pointer implements interface? |
| ----------------- | --------------------------: | ----------------------------: |
| `func (t T) M()`  |                         Yes |                           Yes |
| `func (t *T) M()` |                          No |                           Yes |

## Interfaces

```go
type Clock interface {
	Now() time.Time
}
```

Compile-time assertion:

```go
var _ Clock = (*fakeClock)(nil)
```

## Errors

```go
var ErrNotFound = errors.New("not found")

if err != nil {
	return err
}

if errors.Is(err, ErrNotFound) {
	// handle not found
}
```

## Conditionals

```go
if value == "" {
	return ErrEmpty
}

if err := doWork(); err != nil {
	return err
}
```

## Loops

```go
for _, item := range items {
	// use item
}
```

`_` discards an unused value.

## Time

```go
now.IsZero()
now.Before(other)
now.After(other)
now.Equal(other)
```

Inclusive expiration:

```go
expired := !now.Before(expiresAt)
```

## Context

```go
root := context.Background()

ctx, cancel := context.WithCancel(root)
defer cancel()

timeoutCtx, cancelTimeout := context.WithTimeout(ctx, time.Second)
defer cancelTimeout()
```

```text
WithCancel   -> child that can be explicitly cancelled
WithTimeout  -> child cancelled after a duration
WithDeadline -> child cancelled at a specific time
WithValue    -> child carrying small request-scoped metadata
```

Contexts are immutable. Every `With...` call returns a different child context.

Pass context as the first parameter:

```go
FindByCode(ctx context.Context, code string)
```

## Defer

Schedules a call for when the current function returns:

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()
```

```go
file, err := os.Open(path)
if err != nil {
	return err
}
defer file.Close()
```

Use for function-scoped cleanup. Deferred calls run last-in, first-out.

Avoid accumulating `defer` calls inside long-running loops.

## Table-Driven Tests

```go
tests := []struct {
	name     string
	input    string
	expected bool
}{
	{name: "empty", input: "", expected: false},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		actual := validate(tt.input)
		if actual != tt.expected {
			t.Fatalf("expected %v, got %v", tt.expected, actual)
		}
	})
}
```

## Package Visibility

```go
Link          // exported
NewLink       // exported
status        // package-private
transitionTo  // package-private
```

## Choosing a Construct

| Need                                 | Choose           |
| ------------------------------------ | ---------------- |
| Fixed-shape data                     | Struct           |
| Behavior associated with a type      | Method           |
| Mutate original value                | Pointer receiver |
| Small read-only behavior             | Value receiver   |
| Optional value                       | Often a pointer  |
| Required external behavior           | Interface        |
| Concrete external-system integration | Adapter          |
| Business rules                       | Domain           |
| Orchestration                        | Application      |

## Commands

```powershell
$env:GOCACHE="$PWD\.cache\go-build"
& "C:\Program Files\Go\bin\go.exe" test ./internal/link/...
& "C:\Program Files\Go\bin\go.exe" vet ./internal/link/...
& "C:\Program Files\Go\bin\gofmt.exe" -w <files>
```
