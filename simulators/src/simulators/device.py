"""Single device simulator that wraps a SensorAdapter."""

import asyncio
import logging
from collections.abc import AsyncIterator
from datetime import datetime, timezone

from .adapters.base import SensorAdapter

logger = logging.getLogger(__name__)


class Device:
    """A simulated IoT device that reads from a SensorAdapter at ~1Hz.

    The Device class handles timing, timestamps, and device identity.
    Sensor-specific logic lives in the adapter.
    """

    def __init__(self, device_id: str, adapter: SensorAdapter) -> None:
        self._device_id = device_id
        self._adapter = adapter
        self._running = False

    @property
    def device_id(self) -> str:
        return self._device_id

    async def start(self) -> AsyncIterator[dict]:
        """Yield telemetry dicts at ~1Hz until stopped."""
        self._running = True
        await self._adapter.initialize()

        try:
            while self._running:
                try:
                    reading = await self._adapter.read()
                    yield {
                        "device_id": self._device_id,
                        "timestamp": datetime.now(timezone.utc).isoformat(),
                        "temperature": reading["temperature"],
                        "humidity": reading["humidity"],
                        "soil_moisture": reading["soil_moisture"],
                    }
                except Exception:
                    logger.exception("Adapter read failed for device %s", self._device_id)

                if self._running:
                    await asyncio.sleep(1.0)
        finally:
            await self._adapter.cleanup()

    async def stop(self) -> None:
        """Signal the device to stop after the current tick."""
        self._running = False
