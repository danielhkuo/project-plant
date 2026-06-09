# Load Tests (Step 16)

Performance validation for the ingestion and dashboard APIs.

## Prerequisites

```bash
brew install k6 vegeta        # macOS
```

Bring up the stack first (infra + the Go services):

```bash
make infra-up && make migrate
make run-ingestion &          # :8080
make run-processor &          # consumes Kafka -> Postgres/Redis
make run-dashboard &          # :8081
```

## Targets

| Command | What it does |
|---------|--------------|
| `make load-test-quick` | vegeta: 500 req/s for 30s against `/api/v1/telemetry` — quick smoke |
| `make load-test-full` | k6: ramp 0→500 VUs over 2m, sustain 5m |
| `make load-test-soak` | k6: constant 500 RPS for 30m (memory-stability soak) |
| `make load-test-dashboard` | k6: 50 VUs polling `GET /api/v1/devices` — run with background write load |

`load-test-dashboard` measures read latency *under write pressure*, so generate
background writes in parallel:

```bash
make run-simulators COUNT=200 DURATION=300 &
make load-test-dashboard
```

## Thresholds: gating vs informational

The roadmap SLOs are **p99 < 50ms @ 500 RPS** (ingestion) and **p99 < 100ms** (dashboard reads).

These are **gated only when `STRICT=1`** (set in CI on Linux / native Docker). Locally — especially
on macOS where Docker Desktop's Kafka I/O dominates write latency — k6 still runs and prints the
actual p99/throughput in its summary, but does **not** fail the build. This keeps local runs honest
about the environment while the real SLO is enforced in CI.

```bash
make load-test-full              # local: informational, prints actual p99
STRICT=1 make load-test-full     # CI: thresholds hard-fail the run
```
