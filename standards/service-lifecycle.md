# Service lifecycle

Startup, readiness and shutdown ordering for long-running services.

## Startup and shutdown in main

One place owns the process lifecycle. It creates a context cancelled on `SIGINT` and `SIGTERM`, builds the logger, and keeps an ordered list of shutdown hooks. Everything else receives that context and logger.

Ad-hoc `main` functions tend to grow the same faults. A second cancel context gets wired to the signal context through a helper goroutine, cleanup piles up as `defer`s and servers run on bare goroutines. Shutdown then races and telemetry gets lost.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    var hooks shutdownStack
    log := newLogger()
    hooks.Add(func() { _ = log.Sync() })       // registered first, runs last

    db := openDB(cfg)
    hooks.Add(func() { _ = db.Close() })

    srv := newGRPCServer(log, orders.New(ordersrepo.New(db)))
    hooks.Add(serve(ctx, stop, srv, cfg.Port))  // returns a stop function

    <-ctx.Done()
    time.Sleep(drainPeriod)                     // let the load balancer deregister
    hooks.RunAll(shutdownBudget)
}
```

- Register cleanup on the ordered list. `defer` in `main` runs in an awkward place relative to the drain, because the function blocks.
- Hooks run last-in, first-out, as `defer` does. Register whatever a component depends on ahead of it. Register the database first and the servers that query it afterwards, and shutdown stops the servers before closing the database.
- Registering the telemetry flush and the logger sync before anything else makes them run last, after everything else has finished emitting. Give the trace exporter flush a bounded wait. Skipping it loses spans around every deploy.
- Block on the context at the end of `main` and put nothing after the shutdown sequence.
- A hook returns nothing. It handles or logs its own errors. The runner recovers panics, so one broken hook cannot skip the rest.
- `SIGKILL` cannot be caught, so leave it out of the handled signals.

## Servers return stop functions

Serve helpers start the server on a goroutine and return a `func()` that stops it gracefully and closes the listener. The caller registers that function as a hook.

- Hand the helper the cancel function for the root context. A server that dies unexpectedly triggers it, which brings the entire service down. A half-alive process is worse than a restart.
- Bound every stop. Give gRPC a grace period, then force a stop. Give `http.Server.Shutdown` a deadline.

```go
func stopGRPC(srv *grpc.Server, grace time.Duration) func() {
    return func() {
        done := make(chan struct{})
        go func() { srv.GracefulStop(); close(done) }()
        select {
        case <-done:
        case <-time.After(grace):
            srv.Stop()
        }
    }
}
```

- Prefer the standard gRPC health service on the serving port to a sidecar or an extra port. Report `NOT_SERVING` as soon as the context is cancelled, so the orchestrator stops routing before the drain begins.
- Pass readiness probes for hard dependencies (a cache, a workflow engine) to the health check. A pod that cannot reach something it needs should report not ready.

## Shutdown sequence and timing

1. A signal, or a server failing fatally and calling cancel, ends the root context.
2. Health-checked servers report `NOT_SERVING`.
3. Sleep for a fixed drain period so the load balancer can deregister the pod. Skip the sleep in local runs.
4. Hooks execute newest first.
5. Each stop function enforces its own deadline.

Two limits keep the orchestrator from escalating to `SIGKILL` mid-flush:

- A shared time budget across all hooks. Each hook receives whatever remains. When one overruns, log it, leave it running and continue. After the budget runs out, skip the leftover hooks with a log line.
- A bounded gRPC stop. An unbounded `GracefulStop()` can hang on a stuck stream.

Drain period plus hook budget must sit comfortably below the pod's termination grace period. For a 30 second grace period with a 15 second drain, that leaves roughly 15 seconds to finish in-flight work. Long-running RPCs call for a larger grace period.

Pick one place for the drain sleep. When the process sleeps for the drain itself, a `preStop` hook that sleeps as well doubles the delay and eats the time left for in-flight requests.

## CLIs and one-shot binaries

- Commands that need no signal handling skip the lifecycle machinery. A logger and config load are enough.
- Set the runtime's memory limit and `GOMAXPROCS` from container limits at startup in any binary running under resource limits. A library such as `automaxprocs` does the job.
