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
| `make load-test-slo` | k6: constant 500 RPS for 5m — the ingestion SLO gate (nightly CI runs this) |
| `make load-test-stress` | k6: closed-loop, ramp 0→500 VUs over 2m, sustain 5m — finds the ceiling, never gates |
| `make load-test-soak` | k6: constant 500 RPS for 30m (memory-stability soak) |
| `make load-test-dashboard` | k6: 50 VUs polling `GET /api/v1/devices` — run with background write load |

`load-test-dashboard` measures read latency *under write pressure*, so generate
background writes in parallel:

```bash
make run-simulators COUNT=200 DURATION=300 &
make load-test-dashboard
```

## Open-loop vs closed-loop

The ingestion SLO is a latency target *at a fixed load*, so it has to be measured with a fixed load.

- **Open-loop** (`sustained_throughput.js`, k6 `constant-arrival-rate`): k6 starts `RATE` requests
  per second no matter how fast the server answers, adding VUs if responses slow down. The load stays
  at 500 RPS and latency is the thing being measured. This is the gate.
- **Closed-loop** (`ingestion_load.js`, k6 `ramping-vus`): each of 500 VUs sends its next request as
  soon as the previous one returns, with no sleep. Throughput is whatever the server can absorb
  (about 6,200 RPS on the CI runner), so p99 reflects queueing at saturation, not behaviour at
  500 RPS. Useful for finding the ceiling; it has no thresholds.

`sustained_throughput.js` takes `RATE` and `DURATION` (defaults 500 and `30m`), so the same script
serves as the 5-minute gate and the 30-minute soak.

## Thresholds: gating vs informational

The roadmap SLOs are **p99 < 50ms @ 500 RPS** (ingestion) and **p99 < 100ms** (dashboard reads).

These are **gated only when `STRICT=1`** (set in CI on Linux / native Docker). Locally — especially
on macOS where Docker Desktop's Kafka I/O dominates write latency — k6 still runs and prints the
actual p99/throughput in its summary, but does **not** fail the build. This keeps local runs honest
about the environment while the real SLO is enforced in CI.

```bash
make load-test-slo               # local: informational, prints actual p99
STRICT=1 make load-test-slo      # CI: thresholds hard-fail the run
```
