"""Fleet orchestrator for running multiple simulated devices concurrently."""

import asyncio
import logging
from collections.abc import Awaitable, Callable

from .adapters import SimulatedSensorAdapter
from .device import Device
from .profiles import get_profile

logger = logging.getLogger(__name__)


class Fleet:
    """Orchestrates N concurrent simulated devices.

    Each device runs in its own async task, generating telemetry at ~1Hz.
    Events are dispatched to an optional on_reading callback.
    """

    def __init__(
        self,
        count: int,
        *,
        profiles: list[str] | None = None,
        on_reading: Callable[[dict], Awaitable[None]] | None = None,
    ) -> None:
        self._on_reading = on_reading
        self._running = False
        self.devices: list[Device] = []
        self._tasks: list[asyncio.Task] = []

        profile_list = profiles or ["default"]
        for i in range(count):
            profile = get_profile(profile_list[i % len(profile_list)])
            adapter = SimulatedSensorAdapter(
                temp_range=profile.temp_range,
                humidity_range=profile.humidity_range,
                soil_range=profile.soil_range,
            )
            device_id = f"dev-{i:03d}"
            self.devices.append(Device(device_id, adapter))

    async def start(self, duration: float | None = None) -> None:
        """Run all devices concurrently for the given duration (or until stopped)."""
        self._running = True

        async def run_device(device: Device) -> None:
            try:
                async for reading in device.start():
                    if self._on_reading is not None:
                        try:
                            await self._on_reading(reading)
                        except Exception:
                            logger.exception("on_reading callback failed")
                    if not self._running:
                        await device.stop()
                        break
            except Exception:
                logger.exception("Device %s failed", device.device_id)

        self._tasks = [asyncio.create_task(run_device(d)) for d in self.devices]

        if duration is not None:
            async def stop_after_duration() -> None:
                await asyncio.sleep(duration)
                await self.stop()

            self._tasks.append(asyncio.create_task(stop_after_duration()))

        await asyncio.gather(*self._tasks, return_exceptions=True)

    async def stop(self) -> None:
        """Stop all devices gracefully."""
        self._running = False
        for device in self.devices:
            await device.stop()
        # Cancel any remaining tasks (e.g., duration timer)
        for task in self._tasks:
            if not task.done():
                task.cancel()
        self._tasks.clear()
