# Project Plant — 18-Step TDD Roadmap

Each step follows test-driven development: **write tests first**, then implement to make them pass.

---

## Progress

**Phase 1 — Foundation & Device Layer**
- [x] **Step 1:** Project Scaffolding + Shared Schema
- [x] **Step 2+3:** Single Device Simulator + Device Adapter Interface + SDK + Spec Doc (merged)
- [x] **Step 4:** Environmental Profiles + Fleet Orchestrator (Python)

**Phase 2 — Ingestion Pipeline**
- [x] **Step 5:** Ingestion API — Validation + Handlers (Go)
- [x] **Step 6:** Device Authentication (Go)
- [x] **Step 7:** Kafka Producer Integration (Go + testcontainers)

**Phase 3 — Processing & Storage**
- [x] **Step 8:** Stream Processor — Consumer + Rule Engine (Go)
- [x] **Step 9:** Storage Layer — Postgres + Redis (Go + testcontainers)

**Phase 4 — Dashboard**
- [x] **Step 10:** Dashboard API — REST + WebSocket (Go)
- [x] **Step 11:** Frontend Dashboard (Next.js)

**Phase 5 — Integration & Validation**
- [ ] **Step 12:** Ingestion Integration Tests (HTTP → Kafka)
- [ ] **Step 13:** Processor Integration Tests (Kafka → Store + Alerts)
- [ ] **Step 14:** Simulator Transport + E2E Wiring (Python)
- [ ] **Step 15:** Full System E2E Tests
- [ ] **Step 16:** Load Testing + Performance Validation

**Phase 6 — Production Readiness**
- [ ] **Step 17:** CI/CD Pipeline + Dockerization
- [ ] **Step 18:** Observability — Structured Logging, Metrics, Health Checks

---

## Dependency Graph

```
Step 1 (Scaffolding + Schema)
│
├── Steps 2-4 (Python: simulator + adapter + fleet)  ──┐
├── Steps 5-7 (Go: ingestion API + auth + Kafka)      ──┤── can develop in parallel
├── Steps 8-9 (Go: processor + storage)                ──┘
│
├── Step 10 (Dashboard API)              ── needs Step 9 (storage layer)
├── Step 11 (Frontend)                   ── needs Step 10 (dashboard API)
│
├── Step 12 (Ingestion integration)      ── needs Steps 5-7
├── Step 13 (Processor integration)      ── needs Steps 8-9
├── Step 14 (Simulator transport)        ── needs Steps 2-4 + Steps 5-7
│
├── Step 15 (Full E2E)                   ── needs Steps 12-14
├── Step 16 (Load testing)               ── needs Step 15
├── Step 17 (CI/CD)                      ── needs all above
└── Step 18 (Observability)              ── can start after Step 9, finalize after Step 17
```

Steps 2-4, 5-7, and 8-9 are independent workstreams after Step 1.
Steps 10-11 (dashboard) can develop in parallel with Steps 12-14 (integration tests).

---

## Step 1: Project Scaffolding + Shared Schema ✅

**Tests first:**
- `tests/contract/test_json_schema.py` — 23 tests validating good/bad payloads against `schemas/telemetry_event.json`
- `pkg/telemetry/types_test.go` — 13 tests: JSON roundtrip, field names, 11 validation cases

**Implemented:**
- `schemas/telemetry_event.json` — device_id, timestamp, temperature, humidity, soil_moisture with type/range constraints
- `pkg/telemetry/types.go` — Go struct with JSON tags and `Validate()` method
- Go modules: `pkg/`, `services/ingestion/`, `services/processor/`
- Python projects: `simulators/pyproject.toml`, `tests/pyproject.toml`
- `docker-compose.yml` (Kafka KRaft + Postgres 17 + Redis 8, health checks)
- `docker-compose.test.yml` (ephemeral volumes, tmpfs for Postgres)
- `Makefile` with `test-go-unit`, `test-python-unit`, `test-contract`, `test-all`

**Result:** `make test-all` → 36/36 tests pass (13 Go + 23 Python)

---

## Step 2: Single Device Simulator (Python)

**Goal:** Build the first concrete sensor implementation — a simulated device that generates realistic telemetry readings with natural drift patterns.

**Tests first** (`simulators/tests/unit/test_device.py`):

| Test | What It Validates |
|------|-------------------|
| `test_device_generates_valid_payload` | Output dict has all 5 required keys, values within schema ranges |
| `test_device_generates_at_1hz` | Collect payloads for 3 seconds, assert count is 3 ±1 |
| `test_device_id_is_stable` | Same device always emits the same `device_id` across readings |
| `test_device_payload_matches_schema` | Every generated payload validates against `schemas/telemetry_event.json` |
| `test_device_realistic_drift` | Over 100 consecutive readings, temperature never jumps > 2°C between adjacent readings |
| `test_device_timestamp_is_recent` | Timestamp is within 2 seconds of `datetime.now(UTC)` |
| `test_device_stop` | Device can be stopped gracefully, async generator exits cleanly |

**Implement:**
- `simulators/src/simulators/device.py`:
  - `Device` class with `__init__(self, device_id: str, adapter: SensorAdapter)`
  - `async def start(self) -> AsyncIterator[dict]` — yields telemetry dicts at 1Hz
  - Wraps each `adapter.read()` call with device_id and timestamp
  - Internal state: tracks `_running` flag for graceful shutdown

**Key design decision:** The `Device` class does NOT generate sensor values itself — it delegates to a `SensorAdapter`. This step uses a default `SimulatedSensorAdapter` inline, but Step 3 formalizes the interface.

**Acceptance criteria:**
- All 7 unit tests pass
- `Device("dev-001")` produces a stream of valid, schema-conforming payloads
- No external dependencies needed to run tests

---

## Step 3: Device Adapter Interface + SDK + Spec Doc

**Goal:** Formalize the boundary between "read sensor data" and "send it to the platform" so that swapping in real hardware later requires only implementing a single method.

**Tests first** (`simulators/tests/unit/test_adapter.py`):

| Test | What It Validates |
|------|-------------------|
| `test_simulated_adapter_implements_interface` | `SimulatedSensorAdapter` is a valid `SensorAdapter` subclass |
| `test_adapter_read_returns_required_keys` | `read()` returns dict with exactly `temperature`, `humidity`, `soil_moisture` |
| `test_adapter_read_values_in_range` | All returned values are within schema-defined bounds |
| `test_adapter_is_async` | `read()` is a coroutine (supports I2C/SPI delays on real hardware) |
| `test_custom_adapter_works_with_device` | A hand-written test adapter plugs into `Device` and produces valid payloads |
| `test_adapter_read_error_handling` | If `read()` raises, `Device` logs the error and retries on next tick (doesn't crash) |

**Implement:**

```
simulators/src/simulators/
├── adapters/
│   ├── __init__.py
│   ├── base.py              # SensorAdapter ABC
│   └── simulated.py         # SimulatedSensorAdapter (random walk logic from Step 2)
├── device.py                # Updated to accept any SensorAdapter
└── ...
```

- `adapters/base.py` — Abstract base class:
  ```python
  class SensorAdapter(ABC):
      @abstractmethod
      async def read(self) -> SensorReading:
          """Read current sensor values. Called at ~1Hz by Device."""
          ...

      @abstractmethod
      async def initialize(self) -> None:
          """One-time setup (GPIO init, I2C bus open, calibration)."""
          ...

      @abstractmethod
      async def cleanup(self) -> None:
          """Release hardware resources."""
          ...
  ```
- `adapters/simulated.py` — Moves random walk logic here, implements all three methods
- `SensorReading` — TypedDict or dataclass: `{"temperature": float, "humidity": float, "soil_moisture": float}`

**Write spec doc** (`docs/DEVICE_ADAPTER_SPEC.md`):
- **Protocol overview:** What adapters are, how they plug in
- **Interface reference:** `SensorAdapter` ABC with method signatures, expected return types, error contract
- **Lifecycle:** `initialize()` → repeated `read()` at 1Hz → `cleanup()` on shutdown
- **Payload format:** JSON Schema reference, field ranges, timestamp handling
- **Auth flow:** How devices get API keys, how keys are passed to the transport layer
- **Transport:** The SDK handles HTTP, retry, batching — adapters never touch networking
- **Example adapters:**
  - Minimal 15-line example: static values (for testing)
  - Raspberry Pi + DHT22 sketch (pseudocode showing GPIO reads)
  - ESP32 + capacitive soil sensor sketch (pseudocode showing ADC reads)
- **Testing your adapter:** How to run the adapter test harness against a custom implementation

**Acceptance criteria:**
- All 6 adapter unit tests pass
- `SimulatedSensorAdapter` extracted cleanly, `Device` still passes all Step 2 tests
- `docs/DEVICE_ADAPTER_SPEC.md` is comprehensive enough that someone with a Raspberry Pi could write an adapter without reading any other code
- A developer can implement `SensorAdapter` in < 30 lines and have it work with the full pipeline

---

## Step 4: Environmental Profiles + Fleet Orchestrator (Python)

**Goal:** Enable realistic multi-device simulations with different environmental conditions (tropical greenhouse, arid desert, temperate indoor) and orchestrate 100+ concurrent devices.

**Tests first:**

`simulators/tests/unit/test_profiles.py`:

| Test | What It Validates |
|------|-------------------|
| `test_tropical_profile_ranges` | temp 20-35, humidity 60-95, soil_moisture 40-80 |
| `test_arid_profile_ranges` | temp 25-50, humidity 5-30, soil_moisture 5-25 |
| `test_temperate_profile_ranges` | temp 15-28, humidity 40-70, soil_moisture 30-60 |
| `test_dying_profile_ranges` | temp 30-45, humidity 10-25, soil_moisture 2-15 (guaranteed alerts) |
| `test_default_profile_exists` | `get_profile("default")` never raises |
| `test_unknown_profile_raises` | `get_profile("nonexistent")` raises `ValueError` |
| `test_profile_constrains_1000_readings` | Generate 1000 readings with tropical profile, all within bounds |
| `test_profile_initial_values` | Profile provides sensible starting midpoints, not random |

`simulators/tests/unit/test_fleet.py`:

| Test | What It Validates |
|------|-------------------|
| `test_fleet_creates_n_devices` | `Fleet(count=10)` has exactly 10 devices |
| `test_fleet_concurrent_execution` | Start 5 devices for 2s, collect events, assert ≥ 8 total |
| `test_fleet_unique_device_ids` | All device IDs in a 50-device fleet are unique |
| `test_fleet_mixed_profiles` | `profiles=["tropical", "arid"]` assigns round-robin |
| `test_fleet_callback` | Fleet accepts an `on_reading` callback, called for every event |
| `test_fleet_graceful_shutdown` | `fleet.stop()` stops all devices, no dangling tasks |
| `test_fleet_device_failure_isolation` | One device adapter raising doesn't crash the fleet |

**Implement:**
- `simulators/src/simulators/profiles.py`:
  - `Profile` dataclass with `name`, `temp_range`, `humidity_range`, `soil_range`, `initial_values`
  - `get_profile(name: str) -> Profile` factory with built-in profiles
  - `PROFILES` registry dict for easy extension
- `simulators/src/simulators/fleet.py`:
  - `Fleet` class using `asyncio.TaskGroup` to run N devices concurrently
  - Collects events into an `asyncio.Queue`
  - `async def start(duration: float | None = None)` — run for duration or until stopped
  - `async def stop()` — graceful shutdown of all devices
  - `on_reading: Callable[[dict], Awaitable[None]]` callback hook (used by transport in Step 14)

**Acceptance criteria:**
- All 15 profile + fleet tests pass
- `Fleet(count=100)` starts and produces events from 100 concurrent async tasks without errors
- Fleet handles device failures gracefully (one bad adapter doesn't take down the fleet)

---

## Step 5: Ingestion API — Validation + Handlers (Go, No External Deps)

**Goal:** Build the HTTP entry point that all devices talk to. Pure business logic — no Kafka, no database, no auth yet. Just validate payloads and hand them to a producer interface.

**Tests first:**

`services/ingestion/internal/validation/validator_test.go` — 14-case table-driven test:

| Case | Input | Expected |
|------|-------|----------|
| Valid payload | Complete, in-range JSON | `nil` error, populated `TelemetryEvent` |
| Missing device_id | JSON without `device_id` | Error containing "device_id" |
| Empty device_id | `"device_id": ""` | Error containing "device_id" |
| Temperature too low | `"temperature": -41` | Error containing "temperature" |
| Temperature too high | `"temperature": 81` | Error containing "temperature" |
| Humidity below zero | `"humidity": -1` | Error containing "humidity" |
| Humidity above 100 | `"humidity": 101` | Error containing "humidity" |
| Soil moisture below zero | `"soil_moisture": -0.1` | Error containing "soil_moisture" |
| Soil moisture above 100 | `"soil_moisture": 100.1` | Error containing "soil_moisture" |
| Missing timestamp | No `timestamp` field | Error containing "timestamp" |
| Invalid timestamp format | `"timestamp": "not-a-date"` | Error containing "timestamp" |
| Extra fields allowed | Valid payload + `"extra": "field"` | `nil` error (lenient on ingestion) |
| Null body | `nil` input | Error containing "body" |
| Malformed JSON | `{broken` | Error containing "JSON" |

`services/ingestion/internal/api/handler_test.go`:

| Test | What It Validates |
|------|-------------------|
| `TestIngestHandler_ValidPayload` | POST valid JSON → 202 Accepted, mock producer `.Publish()` called once with correct event |
| `TestIngestHandler_InvalidPayload` | POST invalid JSON → 400 Bad Request with structured `{"error": "...", "field": "..."}` body |
| `TestIngestHandler_MethodNotAllowed` | GET /api/v1/telemetry → 405 |
| `TestIngestHandler_EmptyBody` | POST with empty body → 400 |
| `TestIngestHandler_ProducerError` | Producer returns error → 503 Service Unavailable |
| `TestIngestHandler_ContentType` | POST without `Content-Type: application/json` → 415 Unsupported Media Type |

`services/ingestion/internal/api/middleware_test.go`:

| Test | What It Validates |
|------|-------------------|
| `TestRequestIDMiddleware` | Response has `X-Request-ID` header; if client sends one, it's preserved |
| `TestLoggingMiddleware` | Request completion produces structured log with method, path, status, duration |
| `TestRecoveryMiddleware` | Handler that panics → 500 Internal Server Error (not a process crash) |
| `TestCORSMiddleware` | OPTIONS request returns correct CORS headers (needed for frontend in Step 11) |

**Implement:**
- `validation/validator.go` — `Validate(payload []byte) (TelemetryEvent, error)`
- `api/handler.go` — HTTP handler using `net/http`, calls validator then `EventProducer` interface
- `api/middleware.go` — request ID, logging, panic recovery, CORS middleware chain
- `EventProducer` interface: `Publish(ctx context.Context, event TelemetryEvent) error` + `Close() error`
- Structured error responses: `{"error": "message", "field": "optional_field_name"}`

**Acceptance criteria:**
- All unit tests pass with zero external dependencies (mock producer only)
- Validation rejects all 14 invalid cases with descriptive errors
- Handler returns correct HTTP status codes for all scenarios

---

## Step 6: Device Authentication (Go)

**Goal:** Secure the ingestion endpoint so only registered devices can send data. MVP uses static API keys; interface allows future upgrade to JWT or database-backed auth.

**Tests first** (`services/ingestion/internal/auth/auth_test.go`):

| Test | What It Validates |
|------|-------------------|
| `TestValidAPIKey` | Request with valid `X-API-Key` header passes, device_id injected into context |
| `TestMissingAPIKey` | No header → 401 Unauthorized with `{"error": "missing API key"}` |
| `TestInvalidAPIKey` | Wrong key → 401 with `{"error": "invalid API key"}` |
| `TestExpiredAPIKey` | Expired key → 401 with `{"error": "API key expired"}` |
| `TestDeviceIDMismatch` | Key valid but doesn't match `device_id` in payload → 403 Forbidden |
| `TestAuthMiddleware_PassesThrough` | Authed request reaches the next handler in chain |
| `TestAuthMiddleware_BlocksUnauthed` | Unauthed request never reaches the next handler |
| `TestDeviceIdentityFromContext` | `DeviceIDFromContext(ctx)` returns the authenticated device_id |
| `TestStaticAuthenticator_MultipleMDevices` | Different keys for different devices, each resolves correctly |

**Implement:**
- `auth/authenticator.go` — `Authenticator` interface:
  ```go
  type Authenticator interface {
      Authenticate(apiKey string) (DeviceIdentity, error)
  }
  ```
- `auth/static.go` — `StaticKeyAuthenticator` (keys from env/config, `map[string]DeviceIdentity`)
- `auth/middleware.go` — HTTP middleware that extracts `X-API-Key`, calls authenticator, injects `DeviceIdentity` into `context.Context`
- `auth/context.go` — `DeviceIDFromContext(ctx)` helper

**Acceptance criteria:**
- All 9 auth tests pass
- Auth middleware composes cleanly with handler middleware chain
- Swapping `StaticKeyAuthenticator` for a future DB-backed one requires zero handler changes

---

## Step 7: Kafka Producer Integration (Go + testcontainers)

**Goal:** Connect the ingestion API to Kafka. Prove it can sustain 500+ events/sec with reliable delivery.

**Tests first:**

Unit (`services/ingestion/internal/kafka/producer_test.go`):

| Test | What It Validates |
|------|-------------------|
| `TestProducer_ImplementsInterface` | `KafkaProducer` satisfies `EventProducer` interface (compile-time) |
| `TestProducer_SerializesEvent` | Mock underlying writer, assert message bytes are valid JSON with correct fields |
| `TestProducer_UsesDeviceIDAsKey` | Kafka message key is `device_id` (ensures partition affinity) |

Integration (`services/ingestion/integration_test/kafka_integration_test.go`, `//go:build integration`):

| Test | What It Validates |
|------|-------------------|
| `TestProducer_PublishAndConsume` | Publish 10 events → consumer reads 10, all fields match |
| `TestProducer_MessageOrdering` | Events from same device_id arrive in order (partition key) |
| `TestProducer_BrokerDown_GracefulError` | Stop Kafka → publish → error returned (not panic). Restart → publish succeeds |
| `TestProducer_HighThroughput` | Publish 10,000 events, assert all consumed, measure throughput > 500/sec |
| `TestProducer_BatchCompression` | With snappy compression enabled, messages are smaller than uncompressed |

**Implement:**
- `kafka/producer.go` — Wraps `segmentio/kafka-go` `Writer`
  - Configurable: topic, batch size, batch timeout, compression (snappy), max retries
  - Uses `device_id` as partition key for ordering guarantees
  - `Publish(ctx, event)` → serializes to JSON, writes to Kafka
  - `Close()` → flushes pending writes, closes writer
- `kafka/config.go` — `ProducerConfig` struct, populated from env vars

**Acceptance criteria:**
- Unit tests pass without Docker
- Integration tests pass with Kafka testcontainer
- Throughput test confirms > 500 events/sec
- Broker failure is handled gracefully (error return, not crash)

---

## Step 8: Stream Processor — Consumer + Rule Engine (Go)

**Goal:** Build the brain of the system — consume events from Kafka, evaluate alerting rules, and dispatch results to storage and alert channels.

**Tests first:**

`services/processor/internal/engine/engine_test.go` — table-driven, 12+ cases:

| Rule | Input | Expected Alert |
|------|-------|----------------|
| High temperature | temp=50 | `high_temperature`, severity=warning |
| Critical temperature | temp=70 | `critical_temperature`, severity=critical |
| Low temperature | temp=3 | `low_temperature`, severity=warning |
| Dry soil | soil_moisture=15 | `dry_soil`, severity=warning |
| Critical dry soil | soil_moisture=5 | `critical_dry_soil`, severity=critical |
| Waterlogged | soil_moisture=95 | `waterlogged`, severity=warning |
| Low humidity | humidity=8 | `low_humidity`, severity=info |
| High humidity | humidity=98 | `high_humidity`, severity=info |
| All normal | temp=25, hum=60, soil=45 | No alerts |
| Multiple breaches | temp=50, soil=10 | Two alerts returned |
| Custom rules | User-defined rule set | Correctly evaluated |
| Boundary exact | temp=40.0 exactly | No alert (threshold is > 40, not >=) |

`services/processor/internal/consumer/consumer_test.go`:

| Test | What It Validates |
|------|-------------------|
| `TestConsumer_ProcessesMessage` | Receives message → calls `engine.Evaluate()` → calls `store.Write()` + `store.SetLatest()` |
| `TestConsumer_AlertTriggered` | Engine returns alert → calls `alertPublisher.Publish()` + `store.WriteAlert()` |
| `TestConsumer_SkipsMalformed` | Bad JSON → logged as error, skipped, offset committed (no infinite retry loop) |
| `TestConsumer_CommitsOffset` | After successful processing, consumer offset advances |
| `TestConsumer_StoreError_Retries` | Store returns error → consumer retries with backoff (up to 3 times) |

`services/processor/internal/alert/publisher_test.go`:

| Test | What It Validates |
|------|-------------------|
| `TestPublisher_PublishesToRedis` | Mock Redis client, assert `PUBLISH` on channel `alerts:{device_id}` |
| `TestPublisher_AlertPayloadFormat` | Alert JSON has: `alert_id`, `device_id`, `rule_name`, `severity`, `triggered_at`, `reading` |
| `TestPublisher_AlertID_IsUUID` | `alert_id` is a valid UUID v4 |

**Implement:**
- `engine/engine.go`:
  - `RuleEngine` struct with `rules []Rule`
  - `Rule` struct: `Field string`, `Operator string`, `Threshold float64`, `Name string`, `Severity string`
  - `Evaluate(event TelemetryEvent) []Alert` — iterates rules, returns all triggered alerts
  - Default rules loaded from config/embedded JSON
- `consumer/consumer.go`:
  - Kafka consumer using `kafka-go` reader in consumer group mode
  - Processing loop: read → deserialize → engine.Evaluate → store.Write + store.SetLatest → commit
  - If alerts: alertPublisher.Publish + store.WriteAlert
  - Retry logic with exponential backoff for store errors
- `alert/publisher.go`:
  - Wraps Redis `PUBLISH` command
  - Serializes alert to JSON, publishes to `alerts:{device_id}` channel

**Acceptance criteria:**
- All unit tests pass with mocked dependencies
- Rule engine handles all threshold combinations correctly
- Consumer doesn't crash on malformed messages
- Alert payloads are well-structured with UUIDs

---

## Step 9: Storage Layer — Postgres + Redis (Go + testcontainers)

**Goal:** Build the persistence layer. Postgres for durable history, Redis for real-time state. Prove both handle the expected throughput.

**Tests first** (all integration, `//go:build integration`):

Postgres (`services/processor/internal/store/postgres_test.go`):

| Test | What It Validates |
|------|-------------------|
| `TestPostgres_InsertAndQuery` | Insert one event, query by device_id, fields match |
| `TestPostgres_BulkInsert` | Batch insert 1,000 events, assert row count, measure time < 1s |
| `TestPostgres_QueryByTimeRange` | Insert events across 1 hour, query 15-min window, correct subset |
| `TestPostgres_QueryPagination` | Query with limit + offset, correct page returned |
| `TestPostgres_MigrationsApply` | Run all migrations on fresh DB, all tables + indexes exist |
| `TestPostgres_WriteAlert` | Insert alert, query back, all fields match including JSONB `reading` |
| `TestPostgres_Dedup` | Insert same event twice (same device_id + timestamp) → only one row |
| `TestPostgres_DeviceRegistration` | Insert device record, query back, fields match |
| `TestPostgres_AggregateStats` | Insert 100 events for 5 devices, query avg temp per device, correct |

Redis (`services/processor/internal/store/redis_test.go`):

| Test | What It Validates |
|------|-------------------|
| `TestRedis_SetGetLatest` | Write reading to `device:{id}:latest`, read it back, match |
| `TestRedis_OverwritesPrevious` | Two writes, only latest is returned |
| `TestRedis_TTL` | Key has TTL of 5 minutes (configurable) |
| `TestRedis_GetAllDevices` | Set latest for 10 devices, `GetAllLatest()` returns all 10 |
| `TestRedis_PubSubAlert` | Subscribe to `alerts:dev-001`, publish, received within 1s |
| `TestRedis_PubSubWildcard` | Subscribe to `alerts:*`, receive alerts from any device |

**Implement:**
- `store/postgres.go` — `jackc/pgx/v5` connection pool
  - `InsertEvent(ctx, event)`, `InsertBatch(ctx, []event)` (using `pgx.CopyFrom` for bulk)
  - `QueryByDevice(ctx, deviceID, timeRange, pagination)` → `[]TelemetryEvent`
  - `WriteAlert(ctx, alert)`, `QueryAlerts(ctx, filters)`
  - `RegisterDevice(ctx, device)`, `UpdateLastSeen(ctx, deviceID)`
  - `GetStats(ctx)` → aggregate stats (device count, event count, active alerts)
- `store/redis.go` — `redis/go-redis/v9`
  - `SetLatest(ctx, deviceID, reading)` — `SET` with TTL + `SADD` to device set
  - `GetLatest(ctx, deviceID)` → latest reading
  - `GetAllLatest(ctx)` → map of all device latest readings
  - `PublishAlert(ctx, alert)` — `PUBLISH` to channel
  - `SubscribeAlerts(ctx, deviceID)` → channel of alerts
- SQL migrations in `migrations/`:
  ```sql
  -- 001: telemetry_events table + (device_id, recorded_at DESC) index
  -- 002: devices table + unique device_id
  -- 003: alerts table + (device_id, triggered_at DESC) index
  -- 004: unique constraint on (device_id, recorded_at) for dedup
  ```

**Acceptance criteria:**
- All integration tests pass with testcontainers (Postgres 17, Redis 8)
- Bulk insert 1,000 events < 1 second
- Redis Pub/Sub delivers alerts in < 1 second
- Migrations apply cleanly and are idempotent

---

## Step 10: Dashboard API — REST + WebSocket (Go) ✅

**Goal:** Build the read-side API that the frontend will consume. REST for queries, WebSocket for real-time updates. This is the Verkada Command equivalent — how operators see their fleet.

**Tests first:**

`services/dashboard/internal/api/handler_test.go`:

| Test | Endpoint | What It Validates |
|------|----------|-------------------|
| `TestListDevices` | `GET /api/v1/devices` | Returns JSON array of devices with latest readings from Redis |
| `TestListDevices_Empty` | `GET /api/v1/devices` | Empty fleet → `[]` (not null) |
| `TestGetDevice` | `GET /api/v1/devices/:id` | Returns single device with latest reading |
| `TestGetDevice_NotFound` | `GET /api/v1/devices/unknown` | 404 with error body |
| `TestGetDeviceHistory` | `GET /api/v1/devices/:id/history?from=&to=` | Returns time-series array from Postgres |
| `TestGetDeviceHistory_Pagination` | `?limit=10&offset=20` | Correct page of results |
| `TestListAlerts` | `GET /api/v1/alerts` | Returns alerts, default sorted by triggered_at DESC |
| `TestListAlerts_FilterBySeverity` | `?severity=critical` | Only critical alerts returned |
| `TestListAlerts_FilterByStatus` | `?status=active` | Only unresolved alerts |
| `TestResolveAlert` | `POST /api/v1/alerts/:id/resolve` | Sets `resolved_at`, returns updated alert |
| `TestGetStats` | `GET /api/v1/stats` | Returns `{device_count, total_events, active_alerts, events_per_sec}` |
| `TestCORS` | `OPTIONS /api/v1/devices` | Returns correct CORS headers for frontend |

`services/dashboard/internal/ws/handler_test.go`:

| Test | What It Validates |
|------|-------------------|
| `TestWebSocket_Connect` | Client connects to `/api/v1/ws/events`, receives welcome message |
| `TestWebSocket_ReceivesNewReading` | New reading written to Redis → pushed to connected clients |
| `TestWebSocket_ReceivesAlert` | Alert published to Redis Pub/Sub → pushed to connected clients |
| `TestWebSocket_FilterByDevice` | `?devices=dev-001,dev-002` only receives events for those devices |
| `TestWebSocket_Disconnect` | Client disconnects, no goroutine leak |
| `TestWebSocket_MultipleClients` | 10 clients connected, all receive the same event |

**Implement:**
- New Go module: `services/dashboard/`
  ```
  services/dashboard/
  ├── go.mod
  ├── cmd/dashboard/main.go
  ├── internal/
  │   ├── api/
  │   │   ├── handler.go          # REST handlers
  │   │   ├── handler_test.go
  │   │   └── routes.go           # Route registration
  │   ├── ws/
  │   │   ├── handler.go          # WebSocket handler
  │   │   ├── handler_test.go
  │   │   ├── hub.go              # Connection manager (fan-out)
  │   │   └── client.go           # Per-connection goroutine
  │   └── config/
  │       └── config.go
  └── Dockerfile
  ```
- REST handlers read from Redis (hot state) and Postgres (history/alerts)
- WebSocket hub subscribes to Redis `alerts:*` and `readings:*` Pub/Sub channels
- Hub maintains a set of connected clients, broadcasts events via fan-out
- Uses `gorilla/websocket` or `nhooyr.io/websocket` (stdlib-compatible)
- CORS configured for `localhost:3000` (Next.js dev server)

**Acceptance criteria:**
- All REST handler tests pass with mocked stores
- All WebSocket tests pass
- No goroutine leaks on client disconnect
- API responses match the frontend's expected data shapes

**Result:** 22 unit tests pass (12 REST handlers + 4 middleware + 6 WebSocket). `make test-all` green.

**Implemented:**
- `services/dashboard/internal/api/` — REST handlers with interface-based DI, middleware chain, `DeviceWithStatus` type with computed status mapping to Nothing design system tokens
- `services/dashboard/internal/ws/` — WebSocket hub (fan-out), per-connection client with device filtering, Redis Pub/Sub subscriber bridge
- `services/dashboard/internal/store/` — Redis `DeviceReader` + Postgres `HistoryReader`/`AlertStore`/`StatsReader` adapters
- `services/dashboard/cmd/dashboard/main.go` — Entrypoint with graceful shutdown
- **Prerequisite:** Added `readings:{deviceID}` PubSub publish to processor's `SetLatest` (1-line change)

---

## Step 11: Frontend Dashboard (Next.js)

**Goal:** Build a real-time dashboard that mirrors Verkada Command. Operators see device fleet status at a glance, drill into history, and manage alerts.

**Tests first** — Frontend tests use Vitest + React Testing Library:

| Test | What It Validates |
|------|-------------------|
| `test_device_grid_renders` | DeviceGrid component renders N device cards from mock API data |
| `test_device_card_shows_status` | Card shows device_id, latest temp/humidity/moisture, last-seen timestamp |
| `test_device_card_color_coding` | Normal = green, warning = yellow, critical = red |
| `test_alert_feed_renders` | AlertFeed component renders alert list sorted by time |
| `test_alert_resolve_button` | Clicking "Resolve" calls POST `/api/v1/alerts/:id/resolve` |
| `test_history_chart_renders` | HistoryChart component renders line chart with mock time-series data |
| `test_websocket_updates_grid` | Mock WebSocket pushes a reading → DeviceGrid re-renders with new value |
| `test_stats_bar_renders` | StatsBar shows device count, events/sec, active alerts |

**Implement:**
```
frontend/
├── package.json
├── next.config.js
├── tsconfig.json
├── tailwind.config.ts
├── src/
│   ├── app/
│   │   ├── layout.tsx                # Root layout with nav + stats bar
│   │   ├── page.tsx                  # Device grid (main dashboard)
│   │   ├── devices/[id]/page.tsx     # Device detail + history chart
│   │   └── alerts/page.tsx           # Alert management
│   ├── components/
│   │   ├── DeviceGrid.tsx            # Grid of device status cards
│   │   ├── DeviceCard.tsx            # Single device: latest readings + status
│   │   ├── HistoryChart.tsx          # Time-series line chart (recharts)
│   │   ├── AlertFeed.tsx             # Real-time alert list
│   │   ├── StatsBar.tsx              # Top bar: fleet-wide metrics
│   │   └── StatusIndicator.tsx       # Green/yellow/red dot
│   ├── hooks/
│   │   ├── useDevices.ts             # SWR/React Query hook for REST API
│   │   ├── useAlerts.ts
│   │   ├── useDeviceHistory.ts
│   │   └── useWebSocket.ts           # WebSocket hook for real-time updates
│   ├── lib/
│   │   ├── api.ts                    # Typed API client (fetch wrapper)
│   │   └── types.ts                  # TypeScript types matching Go API
│   └── __tests__/                    # Vitest + React Testing Library
│       ├── DeviceGrid.test.tsx
│       ├── AlertFeed.test.tsx
│       └── ...
├── Dockerfile
└── vitest.config.ts
```

**Key tech choices:**
- **Next.js 15** (App Router) — SSR for initial load, client components for real-time
- **Tailwind CSS** — rapid styling, no custom CSS
- **Recharts** — lightweight, React-native charting for time-series
- **SWR** or **React Query** — data fetching with automatic revalidation
- **Native WebSocket** — wrapped in a custom hook, reconnects on disconnect

**Pages:**
1. **Dashboard** (`/`) — grid of all device cards with live-updating readings. Color-coded status. Stats bar at top (device count, events/sec, active alerts).
2. **Device Detail** (`/devices/:id`) — full history chart (last 1h/6h/24h/7d), current readings, alert history for this device.
3. **Alerts** (`/alerts`) — filterable alert table (by severity, status, device). "Resolve" action. Real-time new alerts appear at top.

**Acceptance criteria:**
- All 8 component tests pass
- Dashboard loads in < 2 seconds with 100 devices
- WebSocket updates device cards in real-time without page refresh
- Alert resolution works end-to-end (button click → API call → UI update)
- Responsive layout works on desktop and tablet

---

## Step 12: Ingestion Integration Tests (HTTP → Kafka)

**Goal:** Prove the full ingestion path works end-to-end with real Kafka. These are the first tests that cross service boundaries.

**Tests first** (`services/ingestion/integration_test/api_integration_test.go`, `//go:build integration`):

| Test | What It Validates |
|------|-------------------|
| `TestAPI_EndToEnd_ValidRequest` | POST valid payload → 202, event appears on Kafka within 2s |
| `TestAPI_EndToEnd_AuthRejection` | POST with bad API key → 401, nothing on Kafka |
| `TestAPI_EndToEnd_ValidationRejection` | POST invalid payload → 400, nothing on Kafka |
| `TestAPI_EndToEnd_100Concurrent` | 100 goroutines POST simultaneously → all 100 on Kafka |
| `TestAPI_Healthcheck` | GET `/health` → 200 with `{"status":"ok","kafka":"connected"}` |
| `TestAPI_Healthcheck_KafkaDown` | Kafka stopped → health returns `{"status":"degraded","kafka":"disconnected"}` |
| `TestAPI_GracefulShutdown` | Send SIGTERM → in-flight requests complete, server exits cleanly |

**Implement:**
- Wire real `KafkaProducer` into HTTP server for integration test mode
- Add `/health` endpoint that pings Kafka
- Finalize `cmd/server/main.go` with graceful shutdown (`signal.NotifyContext`)
- Test helpers: `startTestServer(t, kafkaAddr)`, `consumeFromTopic(t, topic, count, timeout)`

**Acceptance criteria:**
- All 7 integration tests pass with testcontainers
- Server handles concurrent requests without race conditions (`go test -race`)
- Graceful shutdown doesn't drop in-flight requests

---

## Step 13: Processor Integration Tests (Kafka → Store + Alerts)

**Goal:** Prove the processing pipeline is reliable — especially under failure conditions. This validates the Kafka buffering guarantee.

**Tests first** (`services/processor/integration_test/pipeline_integration_test.go`, `//go:build integration`):

| Test | What It Validates |
|------|-------------------|
| `TestPipeline_NormalEvent` | Produce normal event → appears in Postgres + Redis latest |
| `TestPipeline_AlertTriggered` | Produce temp=50 event → alert in Postgres + Redis Pub/Sub |
| `TestPipeline_KafkaBuffering` | Stop Postgres → produce 100 events → Kafka buffers → restart Postgres → all 100 eventually stored |
| `TestPipeline_Idempotency` | Produce same event twice → only one row in Postgres |
| `TestPipeline_ConsumerGroupRebalance` | Start 2 consumers → stop one → remaining picks up all partitions |
| `TestPipeline_RedisDownDoesntBlockWrites` | Redis offline → events still written to Postgres (degraded but functional) |
| `TestPipeline_100Events_AllProcessed` | Produce 100 varied events → all appear in Postgres, all latest readings in Redis |

`services/processor/integration_test/storage_integration_test.go`:

| Test | What It Validates |
|------|-------------------|
| `TestRedisAndPostgres_Consistency` | Write via processor, latest from Redis matches most recent in Postgres |
| `TestAlertStoredInBothStores` | Alert appears in Postgres alerts table AND Redis Pub/Sub |

**Implement:**
- Wire consumer + engine + stores in `cmd/processor/main.go`
- Retry logic: store writes retry 3x with exponential backoff
- Dedup: `ON CONFLICT (device_id, recorded_at) DO NOTHING` in Postgres
- Redis failures logged but don't block Postgres writes (graceful degradation)
- Consumer group: `plant-processor` group with auto-offset management

**Acceptance criteria:**
- All 9 integration tests pass with testcontainers (Kafka + Postgres + Redis)
- Zero events lost during Postgres downtime (Kafka buffers until recovery)
- Duplicate events do not create duplicate rows
- Redis failure doesn't cascade to Postgres writes

---

## Step 14: Simulator Transport + E2E Wiring (Python)

**Goal:** Connect the simulator fleet to the real ingestion API. Complete the data path from simulated device → HTTP → Kafka → Processor → Postgres/Redis.

**Tests first:**

Unit (`simulators/tests/unit/test_transport.py`):

| Test | What It Validates |
|------|-------------------|
| `test_http_transport_sends_post` | Mock httpx, assert POST to correct URL with JSON body |
| `test_http_transport_includes_api_key` | `X-API-Key` header present in request |
| `test_http_transport_retries_on_5xx` | Mock 503 response → 3 retries with exponential backoff |
| `test_http_transport_no_retry_on_4xx` | Mock 400 response → no retry (client error) |
| `test_http_transport_timeout` | Request exceeds timeout → raises, no hang |
| `test_http_transport_batch_mode` | Batch of 10 readings sent as single POST to `/api/v1/telemetry/batch` |

Integration (`simulators/tests/integration/test_e2e_ingestion.py`, requires full stack):

| Test | What It Validates |
|------|-------------------|
| `test_simulator_to_postgres` | 1 device × 5s → ≥4 events in Postgres |
| `test_simulator_to_redis_latest` | 1 device × 3s → Redis key `device:{id}:latest` exists with recent timestamp |
| `test_alert_triggered` | Device with `dying` profile → alert appears in Redis Pub/Sub within 10s |
| `test_fleet_10_devices` | 10 devices × 5s → ≥40 events in Postgres, 10 device keys in Redis |

**Implement:**
- `simulators/src/simulators/transport.py`:
  - `HTTPTransport` class using `httpx.AsyncClient` with retry middleware
  - Configurable: base URL, API key, timeout, retry count, batch size
  - `async def send(self, reading: dict)` — single reading POST
  - `async def send_batch(self, readings: list[dict])` — batch POST
- Wire into Fleet: `fleet.on_reading = transport.send`
- CLI entry point: `python -m simulators --count 10 --profile tropical --api-url http://localhost:8080 --api-key <key>`

**Acceptance criteria:**
- Unit tests pass without Docker
- Integration tests pass with full stack running
- Fleet of 10 devices sends real data through the complete pipeline to Postgres + Redis

---

## Step 15: Full System E2E Tests

**Goal:** Validate the entire system works as a whole under realistic conditions, including failure scenarios. These are the final acceptance tests.

**Tests first** (`tests/e2e/test_full_pipeline.py`):

| Test | What It Validates |
|------|-------------------|
| `test_100_devices_5_minutes` | 100 devices × 5 min → ≥25K events in Postgres, all 100 in Redis, ≥1 alert, no error logs |
| `test_graceful_degradation_postgres` | Stop Postgres mid-test → no events lost → restart → all eventually stored |
| `test_graceful_degradation_redis` | Stop Redis mid-test → Postgres still receives events → restart Redis → latest readings repopulate |
| `test_api_latency_under_load` | 200 devices → measure p99 from simulator perspective → < 50ms |
| `test_dashboard_api_reads` | While 50 devices are streaming → `GET /api/v1/devices` responds < 100ms with all devices |
| `test_websocket_receives_live_updates` | Connect WebSocket → start 5 devices → receive events within 2s |
| `test_alert_lifecycle` | Trigger alert → visible in dashboard API → resolve via API → status updated |

**Implement:**
- `tests/e2e/conftest.py` — pytest fixtures managing docker-compose lifecycle
- `tests/e2e/helpers.py` — query helpers for Postgres, Redis, Dashboard API, WebSocket
- Docker compose brings up: Kafka, Postgres, Redis, Ingestion, Processor, Dashboard

**Acceptance criteria:**
- All E2E tests pass
- System handles 100+ concurrent devices without dropped events
- Graceful degradation confirmed for both Postgres and Redis failures
- Dashboard API performs well under load

---

## Step 16: Load Testing + Performance Validation

**Goal:** Prove the system meets its quantitative success criteria with repeatable, automated load tests.

**Tests:**

`tests/load/k6/ingestion_load.js`:
- **Scenario:** Ramp from 0 to 500 virtual users over 2 minutes, sustain for 5 minutes
- **Thresholds:** p99 latency < 50ms, error rate < 0.1%, throughput > 500 req/s

`tests/load/k6/sustained_throughput.js`:
- **Scenario:** Constant 500 RPS for 30 minutes (soak test)
- **Thresholds:** Same as above + no memory growth > 20% from baseline

`tests/load/k6/dashboard_read.js`:
- **Scenario:** 50 concurrent users polling `GET /api/v1/devices` every 2s while ingestion is under load
- **Thresholds:** p99 < 100ms (reads should stay fast under write pressure)

`tests/load/vegeta/ingestion.vegeta`:
- Quick smoke test: 500 req/s for 30 seconds, report results

**Makefile targets:**
- `make load-test-quick` — vegeta 30s smoke test
- `make load-test-full` — k6 ingestion 5min
- `make load-test-soak` — k6 30min soak
- `make load-test-dashboard` — k6 dashboard reads under load

**Acceptance criteria:**

| Metric | Target |
|--------|--------|
| Ingestion p99 latency | < 50ms at 500 RPS |
| Dashboard read p99 | < 100ms at 50 concurrent readers |
| Redis read p99 | < 20ms |
| Ingestion error rate | < 0.1% sustained for 5 minutes |
| Memory stability | No growth > 20% over 30 minutes |
| Zero data loss | All sent events appear in Postgres |

---

## Step 17: CI/CD Pipeline + Dockerization

**Goal:** Automate everything. Every push runs the full test suite. Every merge to main builds deployable Docker images.

**Implement:**

`.github/workflows/ci.yml`:
```
Jobs (with dependency chain):

lint (go + python + frontend)
    │
    ├── go-unit ──────────── go-integration ───┐
    ├── python-unit ──────── python-integration ┼── e2e
    ├── contract ───────────────────────────────┘
    └── frontend-unit ───────────────────────────── frontend-e2e
                                                        │
                                                   load-test (nightly only)
```

**Dockerfiles** (multi-stage builds):
- `services/ingestion/Dockerfile` — build Go binary → minimal `gcr.io/distroless/static` image
- `services/processor/Dockerfile` — same pattern
- `services/dashboard/Dockerfile` — same pattern
- `frontend/Dockerfile` — `node:22-alpine` build → `nginx:alpine` for static serving
- `simulators/Dockerfile` — `python:3.11-slim` with httpx + pydantic

**docker-compose.yml** updated:
- Add `ingestion`, `processor`, `dashboard` services building from Dockerfiles
- Add `frontend` service
- Full stack starts with `docker compose up`

**CI targets:**
- Unit tests: < 2 min
- Integration tests: < 5 min
- E2E tests: < 10 min
- Full CI: < 15 min
- Load tests (nightly): < 45 min

**Acceptance criteria:**
- CI goes green on all jobs
- All Docker images build successfully
- `docker compose up` starts the entire system from scratch
- Full stack health check passes within 60 seconds of startup

---

## Step 18: Observability — Structured Logging, Metrics, Health Checks

**Goal:** Make the system debuggable and monitorable. When something goes wrong at 2 AM, structured logs and metrics tell you where.

**Tests first:**

| Test | What It Validates |
|------|-------------------|
| `TestStructuredLogFormat` | Log output is valid JSON with `timestamp`, `level`, `service`, `msg` fields |
| `TestRequestLogFields` | HTTP request logs include `method`, `path`, `status`, `duration_ms`, `request_id` |
| `TestMetricsEndpoint` | GET `/metrics` returns Prometheus-format metrics |
| `TestMetrics_EventsIngested` | After 10 POSTs, `events_ingested_total` counter = 10 |
| `TestMetrics_AlertsFired` | After triggering 3 alerts, `alerts_fired_total` counter = 3 |
| `TestHealthEndpoint_Comprehensive` | `/health` returns status of all dependencies (Kafka, Postgres, Redis) |

**Implement:**
- Structured logging with `uber-go/zap` across all Go services
  - Request ID propagated through context
  - Log levels: debug (dev), info (production), error (failures)
- Prometheus metrics via `prometheus/client_golang`:
  - `events_ingested_total` — counter, labels: `status` (accepted/rejected)
  - `events_processed_total` — counter, labels: `result` (success/error)
  - `alerts_fired_total` — counter, labels: `rule`, `severity`
  - `ingestion_latency_seconds` — histogram
  - `kafka_consumer_lag` — gauge
  - `active_websocket_connections` — gauge
- Health endpoints on all services: `/health` returns dependency status
- Optional: Grafana dashboard JSON for visualizing metrics

**Acceptance criteria:**
- All logs are structured JSON (no `fmt.Println` anywhere)
- `/metrics` endpoint returns valid Prometheus scrape output
- `/health` accurately reflects dependency status (tested by stopping containers)
- Request ID flows from ingestion through processor (visible in logs)

---

## Verification Checkpoints

1. **After each step:** `make test-all` — all existing tests must pass
2. **After Step 7:** POST a payload to the running ingestion server, consume from Kafka, confirm data flows
3. **After Step 9:** Send an out-of-range event, verify alert appears in Postgres + Redis Pub/Sub
4. **After Step 11:** Open the frontend, see live device data updating in real-time
5. **After Step 15:** Run 100 simulated devices for 5 minutes, verify all metrics (event count, latency, alert generation, zero data loss)
6. **After Step 16:** k6 confirms p99 < 50ms at 500 RPS, 0% error rate for 5 minutes
7. **After Step 17:** Push to GitHub, CI goes green on all jobs
8. **After Step 18:** Grafana dashboard shows live metrics, structured logs are queryable
