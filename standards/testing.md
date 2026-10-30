# Testing

## Unit tests

- Write code that can be unit tested, and refactor whatever resists.
- Prefer table tests to separate test functions, with `t.Parallel()` called in the body of the loop.
- Use `require`, which stops the test at the first failure. `assert` lets later steps run on bad state.
- Use `t.Context()` for the context. A test about cancellation or timeouts may build its own context.
- Cover the risky path as well as the happy one. Error and edge cases get explicit tests.
- Have mocks return values the real dependency would produce. If the real repository errors on a missing row, a mock returning `nil, nil` lets a broken path through CI.

### Mocks

- Generate mocks (mockery or similar) into a `mocks/` directory beside the test file.
- Use the typed `EXPECT()` API and avoid `On()` with string method names. Renames then break at compile time.
- Pin arguments. A wildcard matcher (`mock.Anything`) turns off argument checking, so the test keeps passing after a refactor starts sending the wrong value. Match concrete values or use a custom matcher. A wildcard is fine for arguments the test does not care about, usually the context.
- Use an ordered-expectations helper when call order matters.
- Business logic is tested with mocked interfaces. Repositories and clients are tested against the real dependency.

```go
func TestService_Get(t *testing.T) {
    tests := []struct {
        name    string
        repoErr error
        wantErr bool
    }{
        {name: "found"},
        {name: "repo error is wrapped", repoErr: errBoom, wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            repo := mocks.NewMockinvoiceRepo(t)
            repo.EXPECT().find(t.Context(), "inv_1").Return(&Invoice{}, tt.repoErr)

            _, err := New(repo).Get(t.Context(), "inv_1")

            if tt.wantErr {
                require.ErrorIs(t, err, errBoom)
                return
            }
            require.NoError(t, err)
        })
    }
}
```

## Database tests

Anything needing a `*sqlx.DB` runs against a real database in a container, started once per package in `TestMain`:

```go
var db *sqlx.DB

func TestMain(m *testing.M) {
    _db, cleanup, err := startTestDB()
    if err != nil {
        fmt.Printf("start test db: %s\n", err)
        os.Exit(1)
    }
    db = _db
    code := m.Run()
    cleanup()
    os.Exit(code)
}
```

- The container is thrown away afterwards, so tests need no record cleanup.
- Load the same schema files the real database uses. Hand-built tables drift.
- Build test identifiers with the production constructors so format validation passes. A hand-typed string that fails the validator tests nothing useful.

## End-to-end tests

E2E tests target a live shared environment, so keep them out of the default unit-test workflow and CI's package selection. Expect real API calls and real data.

- Keep suites opt-in per service, runnable by hand against the shared environment or an isolated sandbox.
- Run suites sequentially when tests depend on shared ordering (orders before refunds). Run independent suites in parallel.
- Create fresh resources per test and defer their cleanup. Each test stands alone, though steps inside one test can depend on each other.
- Static pre-existing accounts are acceptable when sign-up is expensive. Provide a script that resets their state.
- Use `require`.
- Break each test into `t.Run()` subtests whose names begin with Given, When or Then.
- When a rate limit bites, poll with a timeout or back off. A fixed sleep adds time and still flakes. Poll the same way for eventual consistency such as search indexing or async processing.
- Run the suite on a schedule in CI, publish machine-readable results and alert the owning team's channel with failing test names and an error snippet. A manual trigger helps while debugging.

### Feature files

Every test directory holds a Gherkin `.feature` file listing what the tests cover. Reviewers use it to see what is under test.

- One `Scenario` per test function, titled in plain language.
- Steps match the test's `t.Run()` names.
- Adding, renaming or removing tests or steps updates the feature file in the same PR. A new test directory gets a new file.

```gherkin
Feature: Orders - cancellation

  Scenario: Customer cancels an unshipped order
    Given a customer with an unshipped order
    When the customer cancels it
    Then the order is marked cancelled
    Then the customer is refunded
```

## Behaviour matrices

When behaviour depends on several inputs at once (a pricing tier, a region, a flag), write the rules down as a decision matrix and treat it as the source of truth.

- Lay out one table per surface, with inputs on the axes and the expected outcome in each cell. Keep one place for the matrix, ideally next to the code.
- Cover every cell, including the combinations that look obvious. The surprising ones show where a rule is implemented twice.
- Reviewers check any change that touches the behaviour against the matrix. A path producing a different outcome from its cell is a bug in the code or in the matrix, and the PR fixes whichever is wrong.
- List the places where the decision is made. A change to any of them gets checked against the matrix.
- Table-driven tests mirror the matrix one row per cell.

```
| Plan     | Domestic | International |
|----------|----------|---------------|
| Free     | 5.00     | blocked       |
| Standard | 0.00     | 9.00          |
| Premium  | 0.00     | 0.00          |
```
