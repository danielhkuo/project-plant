"""Tests for the Device class — single device simulator."""

import asyncio
import json
from datetime import datetime, timezone
from pathlib import Path

import jsonschema
import pytest

from simulators.adapters import SimulatedSensorAdapter
from simulators.device import Device

SCHEMA_PATH = Path(__file__).resolve().parents[3] / "schemas" / "telemetry_event.json"


@pytest.fixture
def schema():
    with open(SCHEMA_PATH) as f:
        return json.load(f)


@pytest.fixture
def device():
    return Device("dev-test-001", SimulatedSensorAdapter())


class TestDevice:
    """Tests for the Device simulator."""

    async def test_device_generates_valid_payload(self, device):
        """Output dict has all 5 required keys, values within schema ranges."""
        async for payload in device.start():
            assert "device_id" in payload
            assert "timestamp" in payload
            assert "temperature" in payload
            assert "humidity" in payload
            assert "soil_moisture" in payload
            assert -40 <= payload["temperature"] <= 80
            assert 0 <= payload["humidity"] <= 100
            assert 0 <= payload["soil_moisture"] <= 100
            await device.stop()
            break

    async def test_device_generates_at_1hz(self, device):
        """Collect payloads for 3 seconds, assert count is 3 +/- 1."""
        payloads = []
        start = asyncio.get_event_loop().time()
        async for payload in device.start():
            payloads.append(payload)
            elapsed = asyncio.get_event_loop().time() - start
            if elapsed >= 3.0:
                await device.stop()
                break

        assert 2 <= len(payloads) <= 4, f"Expected 3 +/- 1 payloads, got {len(payloads)}"

    async def test_device_id_is_stable(self, device):
        """Same device always emits the same device_id across readings."""
        payloads = []
        async for payload in device.start():
            payloads.append(payload)
            if len(payloads) >= 3:
                await device.stop()
                break

        for p in payloads:
            assert p["device_id"] == "dev-test-001"

    async def test_device_payload_matches_schema(self, device, schema):
        """Every generated payload validates against schemas/telemetry_event.json."""
        payloads = []
        async for payload in device.start():
            payloads.append(payload)
            if len(payloads) >= 5:
                await device.stop()
                break

        for p in payloads:
            jsonschema.validate(instance=p, schema=schema)

    async def test_device_realistic_drift(self):
        """Over 100 consecutive readings, temperature never jumps > 2C between adjacent readings."""
        adapter = SimulatedSensorAdapter()
        device = Device("dev-drift-001", adapter)
        payloads = []
        async for payload in device.start():
            payloads.append(payload)
            if len(payloads) >= 100:
                await device.stop()
                break

        for i in range(1, len(payloads)):
            delta = abs(payloads[i]["temperature"] - payloads[i - 1]["temperature"])
            assert delta <= 2.0, (
                f"Temperature jumped {delta:.2f}C between readings {i - 1} and {i}"
            )

    async def test_device_timestamp_is_recent(self, device):
        """Timestamp is within 2 seconds of datetime.now(UTC)."""
        async for payload in device.start():
            ts = datetime.fromisoformat(payload["timestamp"])
            now = datetime.now(timezone.utc)
            diff = abs((now - ts).total_seconds())
            assert diff < 2.0, f"Timestamp is {diff:.1f}s away from now"
            await device.stop()
            break

    async def test_device_stop(self, device):
        """Device can be stopped gracefully, async generator exits cleanly."""
        count = 0
        async for payload in device.start():
            count += 1
            if count >= 2:
                await device.stop()
                break

        assert count == 2
        # Verify device is stopped — starting again should work
        count2 = 0
        async for payload in device.start():
            count2 += 1
            if count2 >= 1:
                await device.stop()
                break
        assert count2 == 1
