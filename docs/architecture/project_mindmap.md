# TinyURL Code And Design Pattern Map

This is a pattern view of the current codebase. Every name below is a real Go
type, function, package, or file in this repository.

```text
LEGEND

  [P1] design-pattern reference       ---> runtime call/data flow
  [I]  Go interface                   - - > dependency/injection
  [S]  concrete struct                ==> implements an interface
  [D]  domain type


====================================================================================
                         WHOLE CODEBASE PATTERN MAP
====================================================================================

                           cmd/linkd/main.go
                      [P1 COMPOSITION ROOT]
                                  |
                  creates concrete objects, then
                  passes them into constructors
                       [P2 CONSTRUCTOR DI]
                                  |
          +-----------------------+-----------------------+
          |                       |                       |
          v                       v                       v
  HTTP ADAPTERS [P3]      APPLICATION USE CASES     OUTBOUND ADAPTERS [P3]
                                                   
  Handler [S]             CreateGeneratedLink [S]    memory.Repository [S]
  ManagementHandler [S]   RedirectLink [S]           postgres.Repository [S]
  health.Handler [S]      GetManagedLink [S]         memory.IdempotencyStore [S]
          |               ChangeLinkStatus [S]       postgres.IdempotencyStore [S]
          |               ChangeLinkDestination [S]  Base62Generator [S]
          |               ChangeLinkExpiration [S]   SystemClock [S]
          |                       |                   RedirectEventRecorder [S]
          |                       |                            |
          |                       |                            |
          |                       | depends on                 | implements
          |                       v                            v
          |               PORTS / INTERFACES [P4 DEPENDENCY INVERSION]
          |                                                                
          |               LinkRepository [I] <================ Repository adapters
          |               IdempotencyStore [I] <============= Idempotency adapters
          |               CodeGenerator [I] <================ Base62Generator
          |               Clock [I] <======================== SystemClock
          |               RedirectEventRecorder [I] <======== Event recorders
          |                       |
          |                       | use cases coordinate
          |                       v
          |                  DOMAIN MODEL
          |                                                                
          |               Link [D] ---------------- [P5 AGGREGATE ROOT]
          |                 |                                              
          |                 +-- DestinationURL [D] --- [P6 VALUE OBJECT]
          |                 +-- LinkStatus [D] ------- [P7 STATE MACHINE]
          |                 `-- version -------------- [P8 OPTIMISTIC CONCURRENCY]
          |                                                                
          |               RedirectEvent [D] ---------- analytics event data
          |                                                                
          +-----------------------+
                                  |
                  HTTP DTO <-> application request/result
                         [P9 DTO MAPPING]


====================================================================================
                         HOW THE MAIN PATTERNS CONNECT
====================================================================================

[P1] COMPOSITION ROOT + [P2] CONSTRUCTOR DEPENDENCY INJECTION

  main.go
     |
     +-- repository := postgres.NewRepository(pool)
     +-- clock      := system.SystemClock{}
     |
     +-- redirect := application.NewRedirectLink(repository, clock)
     |
     `-- handler  := httpapi.NewHandler(create, redirect, baseURL, options...)

  main.go knows concrete types.
  RedirectLink only stores interfaces:

  RedirectLink [S]
     |-- repository ports.LinkRepository [I]
     `-- clock      ports.Clock [I]

  There is no dependency-injection framework. Constructors perform the wiring.


[P3] ADAPTER PATTERN + HEXAGONAL / PORTS-AND-ADAPTERS

                        application core
                              |
                         required port [I]
                              ^
                              |
                 +------------+------------+
                 |                         |
          memory adapter [S]       Postgres adapter [S]

  Example:

  RedirectLink
      ---> LinkRepository.FindByCode                 interface call
                  |
                  +==> memory.Repository.FindByCode
                  |
                  `==> postgres.Repository.FindByCode ---> pgx ---> Postgres

  The application defines what it needs. Technology adapts itself to that
  contract.


[P4] DEPENDENCY INVERSION + INTERFACE-BASED STRATEGY

  ports.LinkRepository [I]
      Insert
      FindByCode
      Update
          ^
          |
          +== *memory.Repository
          `== *postgres.Repository

  TINYURL_STORAGE chooses the strategy at startup:

      memory   -> inject memory implementations
      postgres -> inject Postgres implementations

  Use-case code does not change.

  Go verifies the implementation by method shape, not `implements`:

      var _ ports.LinkRepository = (*Repository)(nil)


[P5] AGGREGATE ROOT

  Link [D]
    owns:
      code, destination, owner, status,
      creation/update/expiration time, version

    protects changes through behavior:
      Disable, Reactivate, Delete
      UpdateDestination
      SetExpiration, ClearExpiration

  Outside code cannot directly assign private fields.

      application use case
              |
              v
      link.Disable(now)
              |
              v
      Link validates the whole state change
              |
              v
      repository saves the valid aggregate


[P6] VALUE OBJECT + FACTORY VALIDATION

  raw string
      |
      v
  NewDestinationURL(raw)
      |
      +-- malformed / unsupported / missing host -> error
      |
      `-- valid -> DestinationURL [D]

  DestinationURL is defined by its validated value, not by an identity.


[P7] STATE MACHINE

                     Disable
        Active --------------------> Disabled
          ^                            |
          +--------- Reactivate -------+
          |                            |
          +---------- Delete ----------+
                       |
                       v
                    Deleted
                    terminal

  LinkStatus.CanTransitionTo owns allowed transitions.
  Link methods apply the transition and increment version.

  This is a state machine, not the GoF State pattern with separate state objects.


[P8] OPTIMISTIC CONCURRENCY

  Request A reads version 7
  Request B reads version 7

  A changes Link -> version 8
  A: UPDATE ... WHERE version = 7  -> one row -> success

  B changes its old copy -> version 8
  B: UPDATE ... WHERE version = 7  -> zero rows -> ErrVersionConflict

  HTTP connects this to:

      response ETag: "7"
             |
             v
      request If-Match: "7"


[P9] DTO / BOUNDARY MAPPING

  JSON bytes
      -> createLinkHTTPRequest             transport DTO
      -> CreateGeneratedLinkRequest        application command
      -> DestinationURL + Link             domain objects
      -> SQL arguments                     persistence representation
      -> database row

  Returning from Postgres:

  row -> scanLink -> ParseLinkStatus + NewDestinationURL
      -> RehydrateLink -> valid domain.Link

  HTTP, domain, and database shapes do not leak into each other.


[P10] REPOSITORY PATTERN + DATA MAPPER

  LinkRepository [I]
      |
      +-- Insert(Link)
      +-- FindByCode(code) -> Link
      `-- Update(Link, expectedVersion)

  postgres.Repository [S]
      |
      +-- maps Link getters to SQL parameters
      `-- scanLink maps a SQL row back to Link

  The application thinks in aggregates, not tables or SQL.


[P11] APPLICATION SERVICE / USE-CASE OBJECT

  HTTP Handler
      |
      v
  ChangeLinkStatus.Execute
      |
      +-- repository.FindByCode
      +-- check owner and expected version
      +-- link.Disable / Reactivate / Delete
      `-- repository.Update

  The use case coordinates the workflow.
  The domain decides whether the business change is valid.
  The adapter performs external I/O.


[P12] IDEMPOTENCY PATTERN

  POST + Idempotency-Key
      |
      v
  IdempotencyStore.Get(owner, key)
      |
      +-- found    -> return original Link
      |
      `-- missing  -> create Link
                       |
                       v
                     Save(owner, key, Link)

  A retried network request does not intentionally create another short link.


[P13] FUNCTION ADAPTER

  health.Checker [I]
      Check(context.Context) error

  pool.Ping
      has shape: func(context.Context) error
              |
              v
  health.CheckerFunc(pool.Ping)
              |
              | named function type has Check method
              v
  satisfies health.Checker

  CheckerFunc adapts a plain function into an interface implementation.


[P14] FUNCTIONAL OPTIONS

  NewHandler(create, redirect, baseURL,
      WithAnalytics(recorder, clock),
  )

  HandlerOption is:

      type HandlerOption func(*Handler)

  WithAnalytics returns a function that configures optional Handler fields.
  Required dependencies remain constructor parameters.


[P15] HTTP HANDLER ADAPTERS AND METHOD VALUES

  httpapi.Handler [S]
      has ServeHTTP(ResponseWriter, *Request)
              |
              v
      automatically satisfies http.Handler [I]

  ManagementHandler.Get
      bound method value has shape func(ResponseWriter, *Request)
              |
              v
      passed to mux.HandleFunc

  http.ResponseWriter is itself an interface. Passing it by value copies the
  interface pair, but writes still reach the underlying HTTP response object.


[P16] ERROR TRANSLATION

  pgx.ErrNoRows
      -> postgres adapter: ports.ErrLinkNotFound
      -> application: return error
      -> HTTP adapter: 404

  Postgres unique violation
      -> ports.ErrLinkAlreadyExists
      -> HTTP 409

  domain.ErrInvalidTransition
      -> application returns domain error
      -> HTTP 409

  Each boundary translates only the errors it understands.


[P17] CONTEXT PROPAGATION

  *http.Request.Context()
      -> useCase.Execute(ctx)
      -> interface method(ctx)
      -> Postgres adapter
      -> pgx operation

  Cancellation and deadlines travel downward.
  Context is not used for owner IDs or other business arguments.


[P18] TEST DOUBLE / STRATEGY SUBSTITUTION

  production:
      RedirectLink -> LinkRepository [I] -> *postgres.Repository

  test:
      RedirectLink -> LinkRepository [I] -> *fakeRepository

  Same use case, constructor, and interface; only the strategy changes.


====================================================================================
                           REQUESTS ACTIVATE THE PATTERNS
====================================================================================

CREATE
  Handler adapter
    -> DTO mapping
    -> CreateGeneratedLink application service
    -> idempotency
    -> value-object factory
    -> aggregate factory
    -> repository port
    -> Postgres/memory adapter

REDIRECT
  Handler adapter
    -> RedirectLink application service
    -> repository port
    -> Link aggregate CanRedirect
    -> analytics port
    -> HTTP 302

PATCH
  ManagementHandler adapter
    -> DTO + If-Match mapping
    -> mutation application service
    -> repository port
    -> aggregate + state machine
    -> optimistic repository update
    -> HTTP response

READINESS
  health.Handler
    -> Checker interface
    -> CheckerFunc function adapter
    -> pool.Ping


====================================================================================
                         NEXT PATTERNS ADDED BY REDIS
====================================================================================

  RedirectLink
      -> LinkResolver [I]
      -> CacheAsideResolver [S]             [P19] CACHE-ASIDE
           |-- RedirectCache [I] -> Redis adapter
           `-- RepositoryResolver -> LinkRepository -> Postgres

  PutIfNewer(version)                       [P20] ATOMIC CONDITIONAL WRITE
  short Redis timeout + Postgres fallback  [P21] GRACEFUL DEGRADATION / FALLBACK

  Management continues using LinkRepository directly.


====================================================================================
                              FILE ORIENTATION
====================================================================================

cmd/linkd/main.go                    [P1] [P2]
internal/link/domain/                [P5] [P6] [P7] [P8]
internal/link/ports/                 [P4]
internal/link/application/           [P11] [P12] [P17]
internal/link/adapters/httpapi/      [P3] [P9] [P14] [P15] [P16]
internal/link/adapters/postgres/     [P3] [P10]
internal/link/adapters/memory/       [P3] [P18]
internal/health/                     [P13]
cmd/linkd/main.go + config           storage strategy selection
```

The central design is:

```text
Composition root injects adapters into ports
-> application services coordinate work
-> aggregate and value objects protect business validity
-> adapters translate HTTP, memory, Postgres, and later Redis.
```
