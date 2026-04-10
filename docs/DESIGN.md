# Design Document: IoT Plant Telemetry & Alerting System

**Author:** Daniel Kuo
**Target Role:** Backend Engineering Intern, Verkada (Fall 2026)
**Status:** In Development (Steps 1-9 Complete)

---

## 1. Executive Summary & Strategic Intent

**Project Plant** is a distributed, microservices-based backend system designed to ingest, process, and visualize high-frequency telemetry data from simulated environmental sensors, augmented with computer vision diagnostics.

This project is a scaled-down clone of Verkada's enterprise architecture — demonstrating the ability to design distributed, high-concurrency systems that power Verkada's SV-series Environmental Sensors, camera fleets, and the Command web platform.

### Verkada Alignment Matrix

| Project Feature | Verkada Equivalent | Why It Matters |
|---|---|---|
| **Go Ingestion API** | Device heartbeat & telemetry endpoints | Massive concurrency, low-latency network I/O |
| **Kafka Event Streaming** | Distributed message passing for device states | Decoupled systems, no dropped events during spikes |
| **Redis Hot Cache** | Verkada Command Dashboard | O(1) lookups for instant dashboard load times |
| **PostgreSQL Cold Storage** | Historical sensor/video logging | Relational data modeling, time-series storage |
| **Device Adapter SDK** | Camera/sensor firmware abstraction | Hardware-agnostic device integration layer |
| **Real-time Dashboard** | Verkada Command web platform | Live fleet monitoring with WebSocket push |
| **OpenCV Health Analysis** | Hybrid Cloud Edge Analytics (Cameras) | CV alongside sensor data for context-aware alerts |
| **Terraform & Docker** | Infrastructure at Scale | Reproducible microservice deployments |
| **Prometheus Metrics** | Internal observability stack | Production monitoring and alerting |

---

## 2. System Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Device Layer                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │ Simulated   │  │ Raspberry   │  │ ESP32       │  ← SensorAdapter │
│  │ Adapter     │  │ Pi Adapter  │  │ Adapter     │    interface     │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                 │
│         └────────────────┼────────────────┘                         │
│                    Device SDK (transport, auth, retry)               │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ HTTP POST /api/v1/telemetry
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      Ingestion Layer (Go)                            │
│              Auth → Validate → Kafka Producer                        │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ Kafka topic: telemetry.events
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    Processing Layer (Go)                              │
│         Kafka Consumer → Rule Engine → Alert Publisher                │
└─────────┬────────────────┬────────────────┬─────────────────────────┘
          ▼                ▼                ▼
   ┌──────────┐     ┌──────────┐     ┌──────────┐
   │ Postgres │     │  Redis   │     │  Redis   │
   │ (Cold)   │     │ (Hot)    │     │ (Pub/Sub)│
   │ History  │     │ Latest   │     │ Alerts   │
   └─────┬────┘     └─────┬────┘     └─────┬────┘
         └────────────────┼────────────────┘
                          ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    Dashboard Layer                                    │
│  ┌─────────────────────┐     ┌──────────────────────┐               │
│  │ Dashboard API (Go)  │────▶│ Frontend (Next.js)   │               │
│  │ REST + WebSocket    │ WS  │ Device grid, charts, │               │
│  │ Reads from Redis/PG │     │ alerts, real-time     │               │
│  └─────────────────────┘     └──────────────────────┘               │
└──────────────────────────────────────────────────────────────────────┘
```

### Layers

1. **Device Layer:** Hardware-agnostic sensor adapters implementing the `SensorAdapter` interface. The Device SDK handles transport (HTTP), authentication, retry logic, and batching. The simulator is the first adapter; real hardware adapters plug in with zero backend changes.

2. **Ingestion Layer (Go):** REST API gateway that authenticates devices via API key, validates payloads against the JSON schema, and pushes events to a Kafka topic. Stateless and horizontally scalable.

3. **Processing Layer (Go):** Kafka consumer group evaluating telemetry against configurable threshold rules. Writes all events to Postgres (history), updates Redis (latest state), and publishes alerts to Redis Pub/Sub.

4. **Storage & State Layer:**
   - **PostgreSQL 17:** Durable time-series history with deduplication. Indexed by `(device_id, recorded_at)`.
   - **Redis 8:** Latest reading per device (hot cache with TTL) + Pub/Sub channels for real-time alert delivery.

5. **Dashboard Layer:**
   - **Dashboard API (Go):** Read-side REST endpoints for device listing, history queries, alert management. WebSocket endpoint for real-time push to frontend.
   - **Frontend (Next.js):** Real-time device fleet dashboard with live-updating device grid, time-series charts, and alert management. Mirrors Verkada Command.

6. **Vision Edge Service (V2):** Python OpenCV script analyzing plant images for health degradation.

---

## 3. Key Design Decisions

### Why Kafka instead of direct HTTP to Database?

If 1,000 devices send data simultaneously, direct Postgres inserts cause connection pool exhaustion and dropped packets. Kafka acts as a shock absorber — the API acknowledges receipt and hands data to Kafka. Workers consume at their own pace and batch-insert.

**Resilience guarantee:** If Postgres goes down, Kafka retains all events. When Postgres recovers, the consumer replays from the last committed offset. Zero data loss.

### Why Redis + PostgreSQL (CQRS)?

Querying a massive SQL database for the "current state" of 100 sensors on every dashboard load is slow. Redis provides O(1) lookup for current sensor status (hot state), while Postgres handles historical time-series aggregations (cold state). Target: sub-100ms dashboard loads.

Redis also provides Pub/Sub for real-time alert delivery to the Dashboard API's WebSocket connections — no polling required.

### Why a Device Adapter interface?

Real-world IoT platforms support dozens of hardware variants. By defining a `SensorAdapter` ABC with a single `read()` method, we decouple sensor-specific code (GPIO pins, I2C buses, ADC reads) from the transport and platform logic. Adding a new hardware device means implementing one class — the SDK handles everything else.

This mirrors Verkada's approach: cameras and sensors share a common telemetry pipeline despite having very different firmware.

### Why a separate Dashboard API service?

Separating reads from writes follows CQRS. The ingestion API is optimized for high-throughput writes (fire-and-forget to Kafka). The dashboard API is optimized for read patterns (Redis lookups, Postgres aggregations, WebSocket fan-out). They scale independently — you might need 10 ingestion replicas but only 2 dashboard replicas.

### Why Next.js for the frontend?

- **Server-side rendering** for fast initial page load (SEO doesn't matter, but Time-to-Interactive does)
- **React ecosystem** for charting (Recharts) and data fetching (SWR/React Query)
- **TypeScript** for type safety matching the Go API's response shapes
- **App Router** for clean page-based routing (dashboard, device detail, alerts)
- This is a backend-focused project — the frontend needs to be functional and clean, not a design showcase

### Why merge Computer Vision with Telemetry?

Mirrors Verkada's integrated platform. Just as an SV11 vape sensor trigger pulls corresponding camera footage, this system correlates a "low moisture" telemetry alert with a visual "leaf browning" CV event for actionable context.

---

## 4. Device Adapter Architecture

```
┌────────────────────────────────┐
│     Your Hardware Adapter      │  ← You implement this (one class)
│  class RPiDHT22(SensorAdapter) │
│     async def read() -> dict   │
└──────────────┬─────────────────┘
               │ SensorReading
               ▼
┌────────────────────────────────┐
│         Device SDK             │  ← Provided by the platform
│  ┌──────────┐ ┌──────────────┐ │
│  │ Device   │ │ HTTPTransport│ │
│  │ (1Hz     │ │ (auth, retry,│ │
│  │  loop)   │ │  batching)   │ │
│  └──────────┘ └──────────────┘ │
└──────────────┬─────────────────┘
               │ HTTP POST
               ▼
         Ingestion API
```

**SensorAdapter interface:**
```python
class SensorAdapter(ABC):
    async def initialize(self) -> None:  # GPIO init, I2C open, calibration
    async def read(self) -> SensorReading:  # Read current values
    async def cleanup(self) -> None:  # Release hardware resources
```

**What adapter authors DON'T touch:**
- Networking / HTTP / retries
- Authentication
- JSON serialization
- Timestamps
- Error recovery

**Full spec:** See `docs/DEVICE_ADAPTER_SPEC.md` (created in Step 3)

---

## 5. Dashboard API Design

### REST Endpoints

| Method | Path | Description | Source |
|--------|------|-------------|--------|
| `GET` | `/api/v1/devices` | List all devices with latest readings | Redis |
| `GET` | `/api/v1/devices/:id` | Single device detail + latest reading | Redis |
| `GET` | `/api/v1/devices/:id/history` | Historical readings (query params: `from`, `to`, `limit`, `offset`) | Postgres |
| `GET` | `/api/v1/alerts` | List alerts (query params: `severity`, `status`, `device_id`) | Postgres |
| `POST` | `/api/v1/alerts/:id/resolve` | Resolve an alert | Postgres |
| `GET` | `/api/v1/stats` | Fleet-wide stats: device count, events/sec, active alerts | Redis + Postgres |
| `GET` | `/health` | Service health with dependency status | All |
| `GET` | `/metrics` | Prometheus metrics | Internal |

### WebSocket

| Path | Description |
|------|-------------|
| `WS /api/v1/ws/events` | Real-time push of new readings and alerts. Optional query param `?devices=dev-001,dev-002` to filter. |

The WebSocket hub subscribes to Redis Pub/Sub channels and fans out to connected browser clients. Supports automatic reconnection on the frontend side.

---

## 6. Success Criteria & Metrics

| Metric | Target |
|--------|--------|
| **Ingestion throughput** | 500+ events/sec on local hardware |
| **Ingestion API latency** | < 50ms p99 |
| **Dashboard read speed** | < 100ms p99 under write load |
| **Redis read latency** | < 20ms p99 |
| **WebSocket push latency** | < 2s from event ingestion to frontend update |
| **Uptime resilience** | Zero dropped payloads during Postgres downtime |
| **Dashboard initial load** | < 2s with 100 devices |
| **Memory stability** | No growth > 20% over 30-minute soak test |

---

## 7. Testing Ecosystem Design (TDD-First)

### Go Services (Ingestion + Processor + Dashboard)

| Concern | Tool | Notes |
|---------|------|-------|
| Unit tests | `testing` + `testify` (assert/mock) | Table-driven tests, idiomatic Go |
| Struct comparison | `google/go-cmp` | Better than `reflect.DeepEqual` |
| Integration tests | `testcontainers-go` (Kafka, Postgres, Redis modules) | `//go:build integration` tag separates from unit |
| Benchmarks | `testing.B` | In same `_test.go` files |
| Mocking | `testify/mock` on interfaces | `EventProducer`, `Store`, `AlertPublisher` interfaces |
| JSON Schema validation | `santhosh-tekuri/jsonschema` | Cross-language contract checks |
| WebSocket testing | `nhooyr.io/websocket` or `gorilla/websocket` test helpers | Client in test connects and asserts messages |

### Python Services (Simulators + Vision)

| Concern | Tool | Notes |
|---------|------|-------|
| Unit tests | `pytest` + `pytest-asyncio` | `asyncio_mode = "auto"` |
| Coverage | `pytest-cov` | |
| HTTP mocking | `respx` (for `httpx`) | |
| Redis mocking | `fakeredis` | Unit tests only |
| Integration | `testcontainers` (Python) | `@pytest.mark.integration` marker |
| Payload validation | `pydantic` + `jsonschema` | |

### Frontend (Next.js)

| Concern | Tool | Notes |
|---------|------|-------|
| Unit/component tests | Vitest + React Testing Library | Component rendering, event handling |
| API mocking | MSW (Mock Service Worker) | Mock REST + WebSocket in tests |
| E2E (optional) | Playwright | Full browser tests against running stack |

### Cross-Service & System Tests

| Layer | Framework | Infrastructure |
|-------|-----------|---------------|
| Contract | `pytest` + `jsonschema` / `buf breaking` | None |
| E2E | `pytest` + `docker-compose.test.yml` | Full stack |
| Load (primary) | **k6** | Full stack, threshold assertions |
| Load (quick) | **vegeta** | CLI smoke tests |

### Test Organization Convention

```
Go unit:            internal/foo/foo_test.go          (same package, no tag)
Go integration:     integration_test/*_test.go        (//go:build integration)
Python unit:        tests/unit/test_*.py              (default marker)
Python integration: tests/integration/test_*.py       (@pytest.mark.integration)
Frontend unit:      src/__tests__/*.test.tsx           (Vitest)
E2E:                tests/e2e/test_*.py               (@pytest.mark.e2e)
Contract:           tests/contract/test_*.py          (@pytest.mark.contract)
Load:               tests/load/k6/*.js, tests/load/vegeta/*.vegeta
```

---

## 8. Project Structure

```
project-plant/
├── Makefile                              # test-all, lint-all, load-test-quick, etc.
├── docker-compose.yml                    # Full stack local dev
├── docker-compose.test.yml               # Ephemeral test infra
├── .github/workflows/ci.yml
│
├── docs/
│   ├── DESIGN.md                         # This document
│   ├── ROADMAP.md                        # 18-step TDD roadmap
│   └── DEVICE_ADAPTER_SPEC.md            # Hardware adapter specification
│
├── schemas/
│   └── telemetry_event.json              # Shared JSON Schema (single source of truth)
│
├── pkg/                                  # Shared Go library
│   └── telemetry/
│       ├── types.go
│       └── types_test.go
│
├── services/
│   ├── ingestion/                        # Go — REST gateway -> Kafka
│   │   ├── go.mod
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── api/                      # handler, middleware + tests
│   │   │   ├── auth/                     # authenticator + tests
│   │   │   ├── kafka/                    # producer + tests
│   │   │   ├── validation/              # validator + tests
│   │   │   └── config/
│   │   ├── integration_test/
│   │   └── Dockerfile
│   │
│   ├── processor/                        # Go — Kafka consumer -> rules -> storage
│   │   ├── go.mod
│   │   ├── cmd/processor/main.go
│   │   ├── internal/
│   │   │   ├── consumer/                # Kafka consumer + tests
│   │   │   ├── engine/                  # Rule engine + tests
│   │   │   ├── store/                   # postgres, redis + tests
│   │   │   └── alert/                   # publisher + tests
│   │   ├── integration_test/
│   │   └── Dockerfile
│   │
│   ├── dashboard/                        # Go — Read API + WebSocket
│   │   ├── go.mod
│   │   ├── cmd/dashboard/main.go
│   │   ├── internal/
│   │   │   ├── api/                      # REST handlers + tests
│   │   │   ├── ws/                       # WebSocket hub + tests
│   │   │   └── config/
│   │   ├── integration_test/
│   │   └── Dockerfile
│   │
│   └── vision/                           # Python — OpenCV health analysis (V2)
│       ├── pyproject.toml
│       ├── src/vision/
│       └── tests/
│
├── simulators/                           # Python — device simulators + adapter SDK
│   ├── pyproject.toml
│   ├── src/simulators/
│   │   ├── adapters/
│   │   │   ├── base.py                  # SensorAdapter ABC
│   │   │   └── simulated.py             # SimulatedSensorAdapter
│   │   ├── device.py
│   │   ├── fleet.py
│   │   ├── profiles.py
│   │   └── transport.py
│   └── tests/
│       ├── unit/
│       └── integration/
│
├── frontend/                             # Next.js — real-time dashboard
│   ├── package.json
│   ├── next.config.js
│   ├── tsconfig.json
│   ├── src/
│   │   ├── app/                          # Pages: dashboard, device detail, alerts
│   │   ├── components/                   # DeviceGrid, HistoryChart, AlertFeed, etc.
│   │   ├── hooks/                        # useDevices, useAlerts, useWebSocket
│   │   ├── lib/                          # API client, types
│   │   └── __tests__/
│   ├── Dockerfile
│   └── vitest.config.ts
│
├── tests/                                # Cross-service tests
│   ├── e2e/
│   ├── load/k6/
│   ├── load/vegeta/
│   └── contract/
│
├── migrations/                           # Postgres (golang-migrate)
│   ├── 001_create_telemetry.{up,down}.sql
│   ├── 002_create_devices.{up,down}.sql
│   ├── 003_create_alerts.{up,down}.sql
│   └── 004_add_dedup_constraint.{up,down}.sql
│
└── deployments/terraform/
```

---

## 9. Expected Engineering Challenges

1. **Time-Series Data Growth:** Postgres tables grow rapidly. Will need table partitioning or index optimization for performant historical queries.
2. **Idempotency:** If the Kafka consumer crashes and restarts, it must not record duplicate telemetry points. Solved via `ON CONFLICT (device_id, recorded_at) DO NOTHING`.
3. **Cross-Service Communication:** Managing schemas across Go and Python services. Solved via shared JSON Schema as single source of truth with contract tests.
4. **WebSocket Scalability:** Fan-out to many dashboard clients under high event throughput. Solved via Redis Pub/Sub as the broadcast layer — the WebSocket hub subscribes once, fans out to local connections.
5. **Hardware Diversity:** Unknown future sensor hardware. Solved via the `SensorAdapter` interface — adapters only implement `read()`, the SDK handles everything else.
6. **Graceful Degradation:** Redis failure shouldn't block Postgres writes. Postgres failure shouldn't drop events (Kafka buffers). Partial failures are isolated per layer.

---

## 10. Key Dependencies

**Go:** `segmentio/kafka-go`, `jackc/pgx/v5`, `redis/go-redis/v9`, `golang-migrate/migrate/v4`, `stretchr/testify`, `testcontainers-go`, `uber-go/zap`, `nhooyr.io/websocket`, `prometheus/client_golang`

**Python:** `httpx`, `pydantic`, `pytest`, `pytest-asyncio`, `pytest-cov`, `jsonschema`, `testcontainers`, `fakeredis`, `respx`, `ruff`, `mypy`

**Frontend:** `next`, `react`, `typescript`, `tailwindcss`, `recharts`, `swr`, `vitest`, `@testing-library/react`

**Infra:** Docker, docker-compose, k6, vegeta, golangci-lint

### Infrastructure Versions

| Component | Image | Notes |
|-----------|-------|-------|
| **Kafka** | `confluentinc/cp-kafka:7.9.0` | KRaft mode (no Zookeeper) |
| **PostgreSQL** | `postgres:17-alpine` | Latest stable |
| **Redis** | `redis:8-alpine` | Latest stable |
