"""End-to-end tests: simulator fleet -> ingestion -> Kafka -> processor -> stores.

These require the full stack running (infra + ingestion + processor). Bring it
up with, from the repo root:

    make infra-up && make migrate
    make run-ingestion &   # or: cd services/ingestion && go run ./cmd/server
    make run-processor &   # or: cd services/processor && go run ./cmd/processor

Then: cd simulators && .venv/bin/python -m pytest tests/integration -m integration

If the stack is not reachable the tests skip rather than fail.
"""

import os
import time
from types import SimpleNamespace

import pytest

from simulators.fleet import Fleet
from simulators.transport import HTTPTransport

pytestmark = pytest.mark.integration

API_URL = os.getenv("PLANT_API_URL", "http://localhost:8080")
API_KEY = os.getenv("PLANT_API_KEY", "dev-key-001")
PG_DSN = os.getenv("PLANT_PG_DSN", "postgresql://plant:plant@localhost:5432/plantdb")
REDIS_URL = os.getenv("PLANT_REDIS_URL", "redis://localhost:6379")


@pytest.fixture
async def stack():
    """Connect to the running stack, or skip if it is not reachable."""
    httpx = pytest.importorskip("httpx")
    asyncpg = pytest.importorskip("asyncpg")
    aioredis = pytest.importorskip("redis.asyncio")

    try:
        # The ingestion service authenticates all routes, so probe /health with
        # the API key. Any HTTP response means the server is up.
        async with httpx.AsyncClient(timeout=2.0) as client:
            resp = await client.get(f"{API_URL}/health", headers={"X-API-Key": API_KEY})
            resp.raise_for_status()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"ingestion API not reachable at {API_URL}: {exc}")

    try:
        pool = await asyncpg.create_pool(PG_DSN, min_size=1, max_size=4)
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"postgres not reachable at {PG_DSN}: {exc}")

    redis = aioredis.from_url(REDIS_URL, decode_responses=False)
    try:
        await redis.ping()
    except Exception as exc:  # noqa: BLE001
        await pool.close()
        pytest.skip(f"redis not reachable at {REDIS_URL}: {exc}")

    yield SimpleNamespace(pool=pool, redis=redis)

    await pool.close()
    await redis.aclose()


async def _count_events(pool, device_id: str) -> int:
    return await pool.fetchval(
        "SELECT COUNT(*) FROM telemetry_events WHERE device_id = $1", device_id
    )


async def _run_fleet(count: int, profiles: list[str] | None, duration: float) -> None:
    transport = HTTPTransport(API_URL, API_KEY)

    async def on_reading(reading: dict) -> None:
        try:
            await transport.send(reading)
        except Exception:  # noqa: BLE001 -- best effort under test
            pass

    fleet = Fleet(count=count, profiles=profiles, on_reading=on_reading)
    try:
        await fleet.start(duration=duration)
    finally:
        await transport.aclose()
    # Give Kafka + processor time to drain into the stores.
    time.sleep(2.0)


async def test_simulator_to_postgres(stack):
    before = await _count_events(stack.pool, "dev-000")
    await _run_fleet(count=1, profiles=["temperate"], duration=5.0)
    after = await _count_events(stack.pool, "dev-000")
    assert after - before >= 4, f"expected >=4 new events, got {after - before}"


async def test_simulator_to_redis_latest(stack):
    await _run_fleet(count=1, profiles=["temperate"], duration=3.0)
    raw = await stack.redis.get("device:dev-000:latest")
    assert raw is not None, "expected device:dev-000:latest in Redis"

    import json

    payload = json.loads(raw)
    assert payload["device_id"] == "dev-000"
    assert "timestamp" in payload


async def test_alert_triggered(stack):
    # The dying profile keeps soil_moisture in (2, 15), always < 20 -> dry_soil.
    pubsub = stack.redis.pubsub()
    await pubsub.subscribe("alerts:dev-000")
    try:
        await _run_fleet(count=1, profiles=["dying"], duration=5.0)

        deadline = time.time() + 10
        received = None
        while time.time() < deadline:
            msg = await pubsub.get_message(ignore_subscribe_messages=True, timeout=1.0)
            if msg is not None:
                received = msg
                break
        assert received is not None, "expected an alert on alerts:dev-000 within 10s"
    finally:
        await pubsub.unsubscribe("alerts:dev-000")
        await pubsub.aclose()


async def test_fleet_10_devices(stack):
    device_ids = [f"dev-{i:03d}" for i in range(10)]
    before = {d: await _count_events(stack.pool, d) for d in device_ids}

    await _run_fleet(count=10, profiles=["tropical"], duration=5.0)

    total_new = 0
    present_keys = 0
    for d in device_ids:
        total_new += await _count_events(stack.pool, d) - before[d]
        if await stack.redis.get(f"device:{d}:latest") is not None:
            present_keys += 1

    assert total_new >= 40, f"expected >=40 new events across 10 devices, got {total_new}"
    assert present_keys == 10, f"expected 10 device keys in Redis, got {present_keys}"
