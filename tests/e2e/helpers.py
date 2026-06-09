"""Query, load-generation, and assertion helpers for the E2E suite."""

from __future__ import annotations

import asyncio
import json
import os
import time
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

import httpx
import psycopg
import redis.asyncio as aioredis
import websockets

# Load size is tunable so the suite can run quick (CI) or at full roadmap scale.
DEVICES = int(os.getenv("E2E_DEVICES", "100"))
DURATION = float(os.getenv("E2E_DURATION_SECONDS", "300"))
LOSS_TOLERANCE = 0.85  # accept up to 15% in-flight/drain slack on bulk counts


def expected_min_events(devices: int, duration: float) -> int:
    """Lower bound on events for `devices` at ~1Hz over `duration` seconds."""
    return int(devices * duration * LOSS_TOLERANCE)


# ---- Postgres ----------------------------------------------------------------

async def count_events(dsn: str, since: str | None = None) -> int:
    sql = "SELECT COUNT(*) FROM telemetry_events"
    params: tuple = ()
    if since is not None:
        sql += " WHERE recorded_at >= %s"
        params = (datetime.fromisoformat(since),)
    async with await psycopg.AsyncConnection.connect(dsn) as conn:
        async with conn.cursor() as cur:
            await cur.execute(sql, params)
            row = await cur.fetchone()
            return int(row[0])


async def count_alerts(dsn: str, since: str | None = None) -> int:
    sql = "SELECT COUNT(*) FROM alerts"
    params: tuple = ()
    if since is not None:
        sql += " WHERE triggered_at >= %s"
        params = (datetime.fromisoformat(since),)
    async with await psycopg.AsyncConnection.connect(dsn) as conn:
        async with conn.cursor() as cur:
            await cur.execute(sql, params)
            row = await cur.fetchone()
            return int(row[0])


# ---- Redis -------------------------------------------------------------------

async def redis_latest_count(redis_url: str, device_ids: list[str]) -> int:
    client = aioredis.from_url(redis_url)
    try:
        present = 0
        for d in device_ids:
            if await client.get(f"device:{d}:latest") is not None:
                present += 1
        return present
    finally:
        await client.aclose()


# ---- Dashboard REST ----------------------------------------------------------

async def get_json(url: str) -> tuple[Any, float]:
    """GET url; return (parsed JSON, latency_ms)."""
    async with httpx.AsyncClient(timeout=5.0) as client:
        t0 = time.perf_counter()
        r = await client.get(url)
        latency_ms = (time.perf_counter() - t0) * 1000
        r.raise_for_status()
        return r.json(), latency_ms


async def list_devices(dashboard_url: str) -> tuple[list, float]:
    return await get_json(f"{dashboard_url}/api/v1/devices")


async def list_alerts(dashboard_url: str, status: str | None = None) -> list:
    url = f"{dashboard_url}/api/v1/alerts"
    if status:
        url += f"?status={status}"
    data, _ = await get_json(url)
    return data


async def resolve_alert(dashboard_url: str, alert_id: str) -> int:
    async with httpx.AsyncClient(timeout=5.0) as client:
        r = await client.post(f"{dashboard_url}/api/v1/alerts/{alert_id}/resolve")
        return r.status_code


async def probe_ingest_latency(stack, samples: int = 200, interval: float = 0.01) -> list[float]:
    """Measure ingest round-trip latency with a single sequential probe.

    Run this while a background fleet generates load: because the probe issues
    one request at a time (no self-induced client concurrency), each sample is a
    clean server round-trip — the true "API latency under load", isolated from
    the asyncio event-loop contention a 200-coroutine fleet would add.
    """
    latencies: list[float] = []
    headers = {"X-API-Key": stack.api_key, "Content-Type": "application/json"}
    async with httpx.AsyncClient(timeout=5.0) as client:
        for _ in range(samples):
            payload = {
                "device_id": "dev-probe",
                "timestamp": datetime.now().astimezone().isoformat(),
                "temperature": 22.5,
                "humidity": 55.0,
                "soil_moisture": 40.0,
            }
            t0 = time.perf_counter()
            r = await client.post(
                f"{stack.ingest_url}/api/v1/telemetry", json=payload, headers=headers
            )
            r.raise_for_status()
            latencies.append((time.perf_counter() - t0) * 1000)
            await asyncio.sleep(interval)
    return latencies


# ---- WebSocket ---------------------------------------------------------------

async def next_ws_reading(ws, timeout: float) -> dict | None:
    """Read from an open WS connection until a `reading` message or timeout."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            raw = await asyncio.wait_for(ws.recv(), timeout=deadline - time.time())
        except (asyncio.TimeoutError, websockets.ConnectionClosed):
            return None
        msg = json.loads(raw)
        if msg.get("type") == "reading":
            return msg
    return None


# ---- Load generation (real simulator) ----------------------------------------

@dataclass
class FleetHandle:
    sent: int = 0
    latencies_ms: list[float] = field(default_factory=list)


@asynccontextmanager
async def running_fleet(stack, count: int, profiles: list[str] | None = None,
                        measure_latency: bool = False):
    """Run a real device Fleet against ingestion for the duration of the block."""
    from simulators.fleet import Fleet
    from simulators.transport import HTTPTransport

    transport = HTTPTransport(stack.ingest_url, stack.api_key, timeout=5.0, retries=2)
    handle = FleetHandle()

    async def on_reading(reading: dict) -> None:
        t0 = time.perf_counter()
        try:
            await transport.send(reading)
        except Exception:  # noqa: BLE001 — degradation tests expect transient failures
            return
        handle.sent += 1
        if measure_latency:
            handle.latencies_ms.append((time.perf_counter() - t0) * 1000)

    fleet = Fleet(count=count, profiles=profiles, on_reading=on_reading)
    task = asyncio.create_task(fleet.start())
    try:
        yield handle
    finally:
        await fleet.stop()
        await asyncio.gather(task, return_exceptions=True)
        await transport.aclose()


async def run_fleet_for(stack, count: int, duration: float,
                        profiles: list[str] | None = None,
                        measure_latency: bool = False) -> FleetHandle:
    """Run a fleet for `duration` seconds, then let the pipeline drain."""
    async with running_fleet(stack, count, profiles, measure_latency) as handle:
        await asyncio.sleep(duration)
    await asyncio.sleep(2.0)  # Kafka + processor drain
    return handle


# ---- Misc --------------------------------------------------------------------

def p99(values: list[float]) -> float:
    if not values:
        return float("inf")
    ordered = sorted(values)
    idx = min(len(ordered) - 1, int(len(ordered) * 0.99))
    return ordered[idx]


async def eventually(predicate, timeout: float, interval: float = 1.0) -> bool:
    """Poll an async predicate until true or timeout."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        if await predicate():
            return True
        await asyncio.sleep(interval)
    return await predicate()


def assert_no_error_logs(stack) -> None:
    """Fail if any Go service logged an error/fatal during the run."""
    offending: list[str] = []
    for name in ("ingestion", "processor", "dashboard"):
        for line in stack.read_logs(name).splitlines():
            if '"level":"error"' in line or '"level":"fatal"' in line:
                offending.append(f"[{name}] {line}")
    assert not offending, "unexpected error logs:\n" + "\n".join(offending)
