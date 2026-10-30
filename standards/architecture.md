# Architecture

How to split services up and wire them together.

## Organise by domain

- Group code by domain. HTTP versus gRPC describes consumers and has no bearing on where a boundary sits. One process can serve both.
- Name deployables after how they run: `api`, `background-worker`, `scheduled-job`. A directory called `http` or `grpc` suggests the split follows protocol.
- A worker or cron lives with the domain it supports.

### Data domains

- Only reader and writer of their datastore.
- Offer data access operations. Logic stops at integrity constraints (an entity needs a parent, a value is unique).
- Stay inside one domain. Flows that span domains belong a level up.

### Logical domains

- Orchestrate data domains to build product behaviour: ranking, enrichment, filtering by rules owned elsewhere.
- Go through data domains for every read and write.
- Add genuine orchestration. A deployable forwarding each call to a single RPC doubles latency and adds nothing.

A quick test: if swapping the datastore would make you edit it, it is a data domain. When a schema change forces edits in both halves of one directory, two domains are sharing a home and should be split.

## Ports and adapters

Business logic describes its needs as small interfaces declared in its own package. These are the ports. Code that talks to databases, queues and remote services implements them. These are the adapters. `main` joins the two.

| Part | Role | Examples |
|------|------|----------|
| Handlers | Accept an inbound request or message and invoke the logic | HTTP and RPC handlers, queue consumers, cron jobs |
| Interfaces | Describe the dependencies business logic has | `stockChecker`, `orderStore`, `receiptSender` |
| Repositories and clients | Satisfy those interfaces using a database or remote service | SQL repositories, HTTP clients, caches, publishers, throttles |
| `main` | Constructs adapters, picks implementations, injects them | |

Rules of thumb:

- A struct covers one area of responsibility and is named after it (`orders`, `refunds`).
- Reach for a new struct before adding another private helper method. Helpers on one struct cannot be mocked, so any test of the caller exercises all of them, and a single flow gets spread across many tiny functions. If a step deserves a name, it probably deserves its own type behind an interface and its own tests. Comments labelling the stages of a long method point at the same problem.
- When a method acquires an extra duty (a quota check, a fraud screen, a price lookup), split that duty out into its own struct, inject it, and leave the original method as the outline of the flow.
- Put an interface in front of each call that leaves the process. Logic that names a concrete repository, client or throttle cannot be tested in isolation.
- Logic knows nothing about the vendor, transport or queue behind an interface. A second provider arrives as another adapter and the service stays untouched.
- An adapter wraps one real dependency and keeps it in a field. It translates in both directions: driver errors become domain errors and API payloads become shared types. The interface it satisfies lives with the caller.
- Shared types get a package of their own (`types`). Handlers and clients keep wire-format structs unexported and convert at the boundary.
- Mock the interfaces to test logic. Run repositories and clients against a real database or a containerised stand-in.

```go
// package checkout declares what it depends on
type stockChecker interface {
    InStock(ctx context.Context, sku string) (bool, error)
}

type orderStore interface {
    SaveOrder(ctx context.Context, o types.Order) error
}

type Service struct {
    stock  stockChecker
    orders orderStore
}

func New(stock stockChecker, orders orderStore) *Service {
    return &Service{stock: stock, orders: orders}
}

// main.go supplies the concrete adapters
checkoutService := checkout.New(warehouseClient, orderRepo)
```

### Third-party services

Payment processors, mail services, AI models and similar are adapters like any other.

- Declare a capability interface in the calling package and name it for the capability (`Mailer`). The vendor name belongs to the implementation.
- Pick the implementation at startup from a setting such as `MAIL_PROVIDER`. An unrecognised value aborts startup with a clear error.
- Keys, retry policy and response mapping stay inside the implementation.
- Provide a stub implementation for tests and local runs without credentials.

## Startup decisions

- Anything that varies between deployments is a named setting in the service config. `main` reads each one once and passes it in, either as a plain value or by selecting an implementation in a constructor. Below `main`, nothing reads environment variables.
- Avoid branching on the environment's name (`IsProd()`). If there is no way around it, evaluate it once in `main` and pass the result down. Checking it while serving a request creates a path that only runs in production and cannot be tested anywhere else first.
- Test every setting by constructing the struct with that value. Wiring in `main` stays outside unit tests.

```go
func New(client mailClient, mailEnabled bool) *Service {
    var sender receiptSender = discardSender{}
    if mailEnabled {
        sender = newMailSender(client)
    }
    return &Service{sender: sender}
}
```

## Package layout

One deployable's code lives under its own `internal/`:

```
cmd/<deployable>/
    main.go                  # wiring: config, adapters, services, routes, consumers
    internal/
        types/               # shared types
        handlers/<area>/     # HTTP, RPC and queue-message handlers
        service/<area>/      # business logic and the interfaces it needs
        repository/<store>/  # one package per datastore
        client/<vendor>/     # one package per external API
```

- Split handlers and services by feature area, each package declaring its own interfaces. Split repositories by datastore and clients by vendor. Small deployables with a single area can get by with one package per layer. Avoid letting one catch-all service absorb every concern.
- Imports point inwards from `main`. Packages for handlers, services, repositories and clients may import shared types and shared libraries, and never one another. Only `main` sees all of them.
- Same area name in a handler package and a service package? Alias one in `main`.
- An `internal/` one level up holds code shared by several deployables of the same service. Repositories and clients stay with their deployable.
- `main` contains no business logic.
