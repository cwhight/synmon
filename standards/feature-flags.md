# Feature flags

Most changes need no flag. Reach for one to land unfinished work on trunk while dormant, to roll out gradually, or as an operational kill switch.

- Release flags start off. Give each an expiry date and a ticket for removal, and delete it with the old code path soon after full rollout. Set a fixed deadline for that, say 60 days.
- Kill switches start in their safe state, have a descriptive name (`disable-*`, `force-*`) and stay for good.
- Read flags through a single wrapper and keep vendor SDK calls out of feature code. Percentage rollouts need a stable targeting key such as the account id. Missing that, users see the feature flicker between requests.
- Safe to merge and safe to switch on are separate questions. Parity checks, rollback plans and rollout steps decide the second one.

## Select by abstraction

Evaluate a flag where implementations are wired up. Handlers never contain `if flag {}`. Define an interface, write both the legacy and the new implementation to it, and let the flag choose between them. The handler talks to the interface and has no idea a flag exists. That keeps evaluation in one place, lets you test both implementations without faking the flag system, and reduces removal to dropping the losing implementation and the selection code.

- Flags that stay constant while the process runs (environment gates, kill switches, config-driven rollouts) are read once at construction, and the chosen implementation is injected.
- Flags that depend on the request (percentage rollouts, client capability gates) go through a small factory that takes the request, checks the flag and returns an implementation.
- A direct inline check is acceptable when the branch is purely local and an interface would only add noise.

```go
func newSearcher(flags flagReader, legacy, next searcher) searcherFactory {
    return func(ctx context.Context, userID string) searcher {
        if flags.BoolForUser(ctx, "release-new-search", userID) {
            return next
        }
        return legacy
    }
}
```
