"""Tests for the Fleet orchestrator."""

import asyncio


from simulators.adapters import SensorAdapter, SensorReading
from simulators.fleet import Fleet


class TestFleet:
    """Tests for fleet-level device orchestration."""

    async def test_fleet_creates_n_devices(self):
        fleet = Fleet(count=10)
        assert len(fleet.devices) == 10

    async def test_fleet_concurrent_execution(self):
        """Start 5 devices for 2s, collect events, assert >= 8 total."""
        events: list[dict] = []

        async def collect(reading: dict) -> None:
            events.append(reading)

        fleet = Fleet(count=5, on_reading=collect)
        await fleet.start(duration=2.5)

        # 5 devices x 2s @ 1Hz = ~10 events, allow some variance
        assert len(events) >= 8, f"Expected >= 8 events, got {len(events)}"

    async def test_fleet_unique_device_ids(self):
        fleet = Fleet(count=50)
        ids = [d.device_id for d in fleet.devices]
        assert len(set(ids)) == 50

    async def test_fleet_mixed_profiles(self):
        """profiles=["tropical", "arid"] assigns round-robin."""
        fleet = Fleet(count=4, profiles=["tropical", "arid"])
        # Just verify it creates devices without error and has 4
        assert len(fleet.devices) == 4

    async def test_fleet_callback(self):
        """Fleet accepts an on_reading callback, called for every event."""
        received: list[dict] = []

        async def on_reading(reading: dict) -> None:
            received.append(reading)

        fleet = Fleet(count=2, on_reading=on_reading)
        await fleet.start(duration=1.5)

        assert len(received) >= 2

    async def test_fleet_graceful_shutdown(self):
        """fleet.stop() stops all devices, no dangling tasks."""
        fleet = Fleet(count=3)

        async def run_and_stop():
            task = asyncio.create_task(fleet.start(duration=10))
            await asyncio.sleep(1.5)
            await fleet.stop()
            await task

        await asyncio.wait_for(run_and_stop(), timeout=5.0)

    async def test_fleet_device_failure_isolation(self):
        """One device adapter raising doesn't crash the fleet."""

        class FailingAdapter(SensorAdapter):
            async def initialize(self) -> None:
                pass

            async def read(self) -> SensorReading:
                raise RuntimeError("Sensor exploded")

            async def cleanup(self) -> None:
                pass

        events: list[dict] = []

        async def collect(reading: dict) -> None:
            events.append(reading)

        fleet = Fleet(count=3, on_reading=collect)
        # Replace one device's adapter with a failing one
        fleet.devices[0]._adapter = FailingAdapter()

        # Should not crash — the other 2 devices produce events
        await fleet.start(duration=2.5)
        assert len(events) >= 3  # at least some events from the 2 good devices
