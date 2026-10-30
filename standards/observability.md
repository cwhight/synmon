# Observability

Logs, metrics and traces.

## Logging

- Handle each error in one place: return it or log it. A returned error that is also logged gets reported twice.
- Background loops and goroutines have no caller to return to, so they log what they catch and count it (see Metrics).
- Pass the `error` value to the logger and keep `err.Error()` out of it. Structure survives that way, and wrapped errors stay matchable.
- Info and above carry no PII. Treat as PII anything identifying that a user typed or uploaded: contact details, full names, birth dates, IP addresses, free-text bodies, image URLs.
- Opaque server-generated identifiers (account, order, session ids) are not PII. Log them, because following one request across services needs them.

| Situation | Level |
|-----------|-------|
| Successful operation | Info, or Debug on noisy paths |
| Expected failure: validation, missing record | Warn |
| Unexpected failure: bug, broken downstream dependency | Error |

## Metrics

- An error that gets swallowed instead of returned bumps a counter.
- Anything that catches errors internally (workers, crons, goroutines) emits one counter per failure kind. With no counter those failures leave no trace.
- Keep label values to a small fixed set, such as an error kind, a status code or an operation name. Every new value mints another time series, so ids of users, accounts, entities or requests have no place in labels.

```go
// Swallowed, so it needs a counter as well as a log line.
if err := process(ctx, item); err != nil {
    log.Warn("process item", "err", err)
    processFailures.WithLabelValues(item.Kind).Inc()
    continue
}
```

### Alerting

A counter exists to alert on a rate or trend. One transient failure (a network blip, a retried timeout) is normal and gives nobody anything to act on. Alert on an anomalous rate in the metrics system, and keep per-event alerts for failures that are always actionable.

## Tracing

- Keep attributes small and drawn from a limited set of values: counts, flags, short category strings. Integers beat strings.
- Leave out user content, request and response bodies, large structs and unbounded lists.
- Put an identifier on the root span once, for trace lookup. Child spans inherit the trace context and do not need a copy.
- Record the error on the span in failure paths and end the span plainly on success.
- Trace backends index only configured attributes. Ask for indexing on primary search keys and skip diagnostic counts.
