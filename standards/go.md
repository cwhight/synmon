# Go standards

Core rules for any Go change. Specialised areas have their own files, listed in the README.

## Layers

| Layer | Owns |
|-------|------|
| Handler | Turning a protocol request into a service call and the result into a response |
| Service | Business rules and orchestration |
| Repository | Reads and writes against one datastore |
| Client | Calls to one external HTTP API |

Calls go one way: handler -> service -> repository or client. Nothing calls back up.

### Handlers

- Parse and validate input, make one service call, format the response. Anything beyond that (looking something up, branching on business state, calling a second dependency) signals a missing collaborator. Move it behind an interface (see [architecture.md](architecture.md)).
- Shared logic goes in the service. A handler never calls its own sibling methods.
- Use one shared adapter for HTTP handlers. Decoding, encoding, panic recovery and log enrichment then live in a single place and are not repeated per route.
- Convert domain errors to protocol status codes in one helper. Handlers contain no `errors.Is` ladders.
- When the schema declares validation rules and an interceptor enforces them, handler code only checks what the schema cannot express.
- Third-party webhooks that send plain JSON get a plain `http.HandlerFunc` with a private struct describing the payload. The service receives simple values or shared types.
- Read protobuf fields with getters (`req.GetEmail()`). Getters are nil-safe, so no guard clauses are needed.

### Services

- All business rules live here. Handlers, repositories, clients and domain types hold none.
- Service methods are procedural and domain types are plain data (anemic model). One place to read the rules is worth more than clever objects.
- Reach databases and external APIs through repositories and clients only.

### Repositories

- Return data as stored. Filtering, enrichment and rules belong to the service.
- Make no calls to other services.
- Translate driver errors into domain errors. A missing row becomes a not-found error and `sql.ErrNoRows` never leaves the package, so callers need no database imports.

### Clients

- Wrap external HTTP APIs. Generated RPC clients are used directly, since a wrapper adds nothing.
- One package per external API. Decode into private structs that mirror the wire format, then hand back shared types. A second vendor then means a second package and no change to callers.
- Translate the API's failures into domain errors.
- Where the API or datastore supports field selection (masks, projections), request only the fields the caller uses. It shrinks the read and the payload.

## Interfaces

- Define them in the consuming package, unexported, with only the methods that consumer calls.
- When an interface has several implementations, the constructor takes a simple flag or config value and picks one. `main` then never names the interface.
- Generated RPC service interfaces are the one exception, and sit with the provider.

```go
// Provider-side: every consumer is forced to depend on the full surface.
package billing

type Client interface {
    Invoice(ctx context.Context, id string) (*Invoice, error)
    Refund(ctx context.Context, id string) error
}

// Consumer-side: the reporting package declares the one method it needs.
package reporting

type invoiceFinder interface {
    Invoice(ctx context.Context, id string) (*billing.Invoice, error)
}
```

## Errors

Each layer wraps with the call it just made. The message names what was attempted one level down, so the chain reads like a stack trace in plain English.

```go
func (r *invoiceRepo) find(ctx context.Context, id string) (*Invoice, error) {
    var inv Invoice
    if err := r.db.GetContext(ctx, &inv, findInvoiceSQL, id); err != nil {
        return nil, fmt.Errorf("select invoice: %w", err)
    }
    return &inv, nil
}

func (s *service) Get(ctx context.Context, id string) (*Invoice, error) {
    inv, err := s.repo.find(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("find %s: %w", id, err)
    }
    return inv, nil
}
```

- Start messages with a lowercase verb in its base form: `"select invoice: %w"`, `"encode payload: %w"`. Skip "failed to", since every wrapped error is already a failure.
- Include whatever identifies the operation, and avoid naming the Go method.
- Wrap with `%w` so `errors.Is` and `errors.As` keep working.
- Handle each error once. Logging it and also returning it reports it twice (see [observability.md](observability.md)).

## Code style

- Functions: aim for 50 lines or fewer and split anything past 100.
- Each function does one job. Split any function that orchestrates, validates and transforms.
- Nest at most 3 levels deep. Use guard clauses and early returns.
- Names: `i` and `j` are fine for loop indices. Everything else gets a descriptive name. Skip vague names like `data`, `result` and `temp`.
- Extract repeated logic into a helper.
- One mechanism per concern. Fix or extend the existing rate limiter, cache or retry loop before adding a second. Two of them on one call mean two rulebooks to reconcile.
- Let the linter own line length and formatting.
- Give each exported function a file of its own, with the `_test.go` beside it. Files stay short enough to review and a missing test is easy to spot. Unexported helpers sit next to their caller.
- Comments carry the reason the code cannot show. Delete any that narrate the next line. AI-generated step-by-step commentary falls in that group.

## Anti-patterns

| Smell | Fix |
|-------|-----|
| Handler calls its own methods | Move the shared code into the service |
| Interface declared next to its implementation | Declare it where it is consumed |
| Filtering or enrichment in a repository | Move it to the service |
| Handler coordinating several services | Extract the flow into the service layer |
| Goroutine with no panic recovery | See [concurrency.md](concurrency.md) |
| Datastore pool opened without protection | See [database.md](database.md) |

## Pull requests

- Size each PR to stand alone: about 400 hand-written lines and 10 files at most, with generated code and lock files left out of the count. The system keeps working once it merges.
- Split oversized work into an ordered stack. A new API ships with its first caller, so nothing unused lands. Refactors get their own PR, apart from behaviour changes.
- Keep unrelated churn out. Stray reformatting hides the real change.
