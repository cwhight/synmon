# Databases

Connections and schema.

## Connections

Every pool gets a circuit breaker plus a default query timeout. Without them, a degraded database can soak up all connections while nothing signals the problem to alerting.

- Build the pool and apply the breaker in one constructor, so no caller can open an unprotected one.
- Repositories take the protected type, so every query passes through the breaker.
- Name the breaker for the database it guards. Metrics and logs carry that name, and several services can share one database.
- The default timeout only kicks in when the query's context carries no deadline. Callers can override it per query, and zero turns it off.
- When you touch code that opens a raw pool, wrap it. Do not add new raw ones.

```go
// withDefaultTimeout applies d only when the caller has not set a deadline.
func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
    if _, ok := ctx.Deadline(); ok {
        return ctx, func() {}
    }
    return context.WithTimeout(ctx, d)
}

func (p *pool) getContext(ctx context.Context, dest any, q string, args ...any) error {
    ctx, cancel := withDefaultTimeout(ctx, p.defaultTimeout)
    defer cancel()
    _, err := p.breaker.Execute(func() (any, error) {
        return nil, p.db.GetContext(ctx, dest, q, args...)
    })
    return err
}
```

## Schema

### Enum values are stored as integers

Persist protobuf enum values as their numbers, in unsigned integer columns (`TINYINT UNSIGNED`, or `SMALLINT UNSIGNED` once the enum may pass 255). Skip database `ENUM(...)` types holding strings.

Put the enum's full proto name in the column comment. Readers then consult the proto for the mapping, which cannot drift:

```sql
`channel` TINYINT UNSIGNED NOT NULL COMMENT 'shop.NotificationChannel',
```

- A new enum value needs no migration, because the column already accepts it.
- Generated enum mapping code gives the safety a string `ENUM` only gives in strict mode. In non-strict mode an invalid value quietly becomes an empty string with a warning.
- It removes string-to-proto mapping code. Strings buy no speed either, since the database stores an `ENUM` as an integer.
- Refer to values in Go by their generated names. A literal `2` is unreadable.
- Do not list the values in the column comment. It goes stale on the next addition and fixing it costs a migration.
- The price is that hand-written queries read worse (`WHERE channel = 1`). The schema is not tuned for people typing SQL. Check the proto, or generate a readable query.

## Layering reminders

- Repositories own all SQL and return domain errors (see [go.md](go.md)).
- Business logic never sees a driver type. It talks to the repository through a consumer-side interface.
- Test repositories against a real database in a container (see [testing.md](testing.md)).
