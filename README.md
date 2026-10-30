# synmon

Synthetic monitoring bot for distributed systems. Runs configurable HTTP test scenarios through [Tracetest](https://tracetest.io/), validates distributed traces, and exposes Prometheus metrics.

## Install

### Go binary

```bash
go install github.com/cwhight/synmon/cmd/synmon@latest
```

### Docker

```bash
docker build -t synmon:latest https://github.com/cwhight/synmon.git
```

### Clone and build

```bash
git clone git@github.com:cwhight/synmon.git
cd synmon
make build
```

## Integrate into your project

### Step 1: Write your test definitions

Create a `synmon/tests.yaml` in your project:

```yaml
tests:
  - id: health-check
    name: "Health Check"
    method: GET
    url: "${TARGET_BASE_URL}/health"
    headers:
      Content-Type: application/json
    specs:
      - selector: 'span[name = "GET /health"]'
        assertions:
          - "attr:tracetest.selected_spans.count > 0"

  - id: create-order
    name: "Create Order"
    method: POST
    url: "${TARGET_BASE_URL}/api/orders"
    body: '{"item": "test-item", "quantity": 1}'
    headers:
      Content-Type: application/json
      Authorization: "Bearer ${AUTH_TOKEN}"
    specs:
      - selector: 'span[name = "POST /api/orders"]'
        assertions:
          - "attr:http.status_code = 201"
```

Environment variables (`${VAR}`) are expanded at runtime.

### Step 2: Run synmon

```bash
TRACETEST_API_ENDPOINT=http://localhost:11633 \
TARGET_BASE_URL=https://api.example.com \
AUTH_TOKEN=your-token \
TESTS_FILE=synmon/tests.yaml \
TEST_INTERVAL=5m \
synmon
```

### Step 3: Deploy to Kubernetes

Copy the manifests from this repo's `deploy/kubernetes/` directory into your project, or reference them directly:

```bash
# Fetch the manifests
mkdir -p deploy/synmon
for f in namespace.yaml configmap.yaml deployment.yaml service.yaml kustomization.yaml; do
  gh api repos/cwhight/synmon/contents/deploy/kubernetes/$f --jq '.content' | base64 -d > deploy/synmon/$f
done
```

Then customise:
1. Edit `configmap.yaml` — add your test definitions
2. Edit `deployment.yaml` — set your `TARGET_BASE_URL`, `AUTH_TOKEN`, and `TRACETEST_API_ENDPOINT`
3. Apply: `kubectl apply -k deploy/synmon/`

### Step 4: Add Prometheus alerts

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

## How it works

```
synmon (every N min)
  → PUT test definitions to Tracetest
  → POST /api/tests/{id}/run
  → Poll until FINISHED/FAILED
  → Emit Prometheus metrics (pass/fail/error + duration)

Tracetest
  → Fires the HTTP request with traceparent header
  → Collects spans via OTLP
  → Evaluates span assertions
```

## Configuration

All settings via env vars. Optional YAML config file overlay via `CONFIG_FILE`.

| Variable | Required | Default | Description |
|---|---|---|---|
| `TRACETEST_API_ENDPOINT` | yes | - | Tracetest API base URL |
| `TEST_INTERVAL` | no | `5m` | Duration between test cycles |
| `TESTS_FILE` | no | - | Path to YAML test definitions |
| `CONFIG_FILE` | no | - | Path to YAML config file |
| `METRICS_ADDR` | no | `:2112` | Prometheus metrics endpoint |
| `TEST_RUN_TIMEOUT` | no | `60s` | Max wait for a single test run |
| `TEST_POLL_INTERVAL` | no | `2s` | Tracetest polling interval |
| `LOG_LEVEL` | no | `info` | debug, info, warn, error |

## Test definition format

Each test in `tests.yaml` needs:

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique ID (used in Tracetest + Prometheus labels) |
| `name` | yes | Human-readable name |
| `method` | yes | HTTP method |
| `url` | yes | Target URL (supports `${ENV_VAR}` expansion) |
| `body` | no | Request body |
| `headers` | no | Request headers map |
| `specs` | no | Span selector + assertions list |

Global auth headers can be set in the config file under `auth_headers:` — they're added to every test request.

## Prometheus metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `synthetic_monitor_test_runs_total` | Counter | `test`, `result` | Total runs. `result`: pass, fail, error |
| `synthetic_monitor_test_duration_seconds` | Histogram | `test` | Duration per test run |

## Local development

```bash
make compose-up      # Full stack: Tracetest + Postgres + synmon + Prometheus
make compose-down    # Tear down
make build           # Build binary
make test            # Run tests
make lint            # golangci-lint
make docker-build    # Build Docker image
```

## Prerequisites

- [Tracetest](https://tracetest.io/) v1.x
- Your services instrumented with [OpenTelemetry](https://opentelemetry.io/), exporting spans to Tracetest's OTLP endpoint (`:4317`)
