# Concurrency and workers

## Recover panics in goroutines you start

A panic in a goroutine with no recovery takes down the whole process. Every goroutine you start recovers and captures its panic, as a log line and a counter or as a returned error. Ideally wrap this in a small shared package (e.g. `pkg/safego`) so every call site reuses one helper and nobody writes the `defer recover()` by hand.

```go
// fireAndForget runs f on its own goroutine and turns a panic into a log line and a counter.
func fireAndForget(log *slog.Logger, name string, f func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                panics.WithLabelValues(name).Inc()
                log.Error("goroutine panicked", "name", name, "panic", r, "stack", string(debug.Stack()))
            }
        }()
        f()
    }()
}
```

Goroutines run through `errgroup` need the same guard, with the panic converted into the error the group returns:

```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("sync accounts: panic: %v\n%s", r, debug.Stack())
        }
    }()
    return syncAccounts(ctx)
})
return g.Wait()
```

- Every goroutine has an owner that cancels it through a context and waits for it on shutdown.
- A goroutine that swallows an error also counts it (see [observability.md](observability.md)).

## Workers, crons and consumers

### Choose failure semantics on purpose

Each handler decides what failure means, and the code makes the choice visible. Neither option is a default.

- Best-effort: swallow the failure, log it, count it and move on. Suits batches where a single poisoned item should never block its neighbours. An error returned from such a handler redelivers the entire batch.
- Retry: return an error and let the framework redeliver. Suits items that have to succeed eventually.

Comments and code must agree. A note saying errors are deliberately swallowed to avoid a retry loop is only true if every branch after it really swallows.

### Expect the same input twice

Redelivery, overlapping cron runs and restarts mean any handler can see the same input more than once.

- Order side effects so a failure part-way is safe to retry. A retry restarts at the top, so a crash between step one and step two repeats step one.
- Protect every side effect that is unsafe to repeat with a dedup key, a unique constraint or a state check. Double charges, duplicate notifications and double writes come from skipping this.
