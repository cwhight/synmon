# Synmon

Synthetic monitoring bot. Runs HTTP test scenarios through Tracetest, validates distributed traces, exposes Prometheus metrics.

## Repo Structure

- `cmd/synmon/` — Application entrypoint
- `internal/config/` — Configuration loading (env vars + YAML overlay)
- `internal/monitor/` — Core monitoring loop, test scenario loading
- `internal/tracetest/` — Tracetest REST API client
- `deploy/kubernetes/` — K8s manifests (Deployment, ConfigMap, Service, Kustomize)
- `examples/` — Example test definitions, config, and Prometheus config
- `standards/` — Code standards (synced from cwhight/go-standards)

## Code Standards

Single source of truth: `standards/go.md` and `standards/testing.md`, plus the topic files in `standards/` that match the area being changed.
Sync with `make sync-standards`. All Go code must conform.

## Running

```bash
make build           # Build binary
make compose-up      # Full local stack
make test            # Run tests
make lint            # golangci-lint
```

## For AI agents integrating synmon into another project

If you are an AI agent working in a different repo and need to set up synmon for that project, follow these steps:

### 1. Install the binary

```bash
go install github.com/cwhight/synmon/cmd/synmon@latest
```

### 2. Create test definitions

Create a `synmon/tests.yaml` in the consuming project. Each test needs an `id`, `name`, `method`, `url`, and optional `specs` with span assertions. Use `${ENV_VAR}` syntax for secrets and dynamic values.

Fetch the example for reference:

```bash
gh api repos/cwhight/synmon/contents/examples/tests.yaml --jq '.content' | base64 -d
```

### 3. Add Kubernetes manifests (if deploying to k8s)

Fetch the manifests into the consuming project:

```bash
mkdir -p deploy/synmon
for f in namespace.yaml configmap.yaml deployment.yaml service.yaml kustomization.yaml; do
  gh api repos/cwhight/synmon/contents/deploy/kubernetes/$f --jq '.content' | base64 -d > deploy/synmon/$f
done
```

Then customise:
- `configmap.yaml` — replace the test definitions with the project's tests
- `deployment.yaml` — set `TRACETEST_API_ENDPOINT`, `TARGET_BASE_URL`, and any auth env vars

### 4. Add to docker-compose (if running locally)

Fetch the docker-compose for reference:

```bash
gh api repos/cwhight/synmon/contents/docker-compose.yml --jq '.content' | base64 -d
```

Add the `synmon` service to the consuming project's docker-compose, pointing `TESTS_FILE` at the project's test definitions.

### 5. Add Prometheus alerts

```yaml
groups:
  - name: synmon
    rules:
      - alert: SyntheticTestFailing
        expr: increase(synthetic_monitor_test_runs_total{result="fail"}[15m]) > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Synthetic test {{ $labels.test }} is failing"
```

### Key env vars for the consuming project

| Variable | Required | Description |
|---|---|---|
| `TRACETEST_API_ENDPOINT` | yes | Tracetest API base URL |
| `TESTS_FILE` | yes | Path to the project's tests.yaml |
| `TARGET_BASE_URL` | depends | Base URL of the service under test (used in test YAML) |
| `TEST_INTERVAL` | no | How often to run (default 5m) |
