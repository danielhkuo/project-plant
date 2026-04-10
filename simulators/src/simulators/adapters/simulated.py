"""Simulated sensor adapter with random-walk drift."""

import random

from .base import SensorAdapter, SensorReading

# Default ranges from the JSON schema
DEFAULT_TEMP_RANGE = (-40.0, 80.0)
DEFAULT_HUMIDITY_RANGE = (0.0, 100.0)
DEFAULT_SOIL_RANGE = (0.0, 100.0)


class SimulatedSensorAdapter(SensorAdapter):
    """Generates realistic sensor readings using random-walk drift.

    Each call to read() adjusts previous values by a small normally-distributed
    delta, clamped to the configured range. This produces realistic time-series
    data with natural drift patterns.
    """

    def __init__(
        self,
        *,
        temp_range: tuple[float, float] = DEFAULT_TEMP_RANGE,
        humidity_range: tuple[float, float] = DEFAULT_HUMIDITY_RANGE,
        soil_range: tuple[float, float] = DEFAULT_SOIL_RANGE,
    ) -> None:
        self._temp_range = temp_range
        self._humidity_range = humidity_range
        self._soil_range = soil_range
        self._temperature: float = 0.0
        self._humidity: float = 0.0
        self._soil_moisture: float = 0.0

    async def initialize(self) -> None:
        """Set initial values at midpoints of configured ranges."""
        self._temperature = sum(self._temp_range) / 2
        self._humidity = sum(self._humidity_range) / 2
        self._soil_moisture = sum(self._soil_range) / 2

    async def read(self) -> SensorReading:
        """Read current sensor values with random-walk drift."""
        self._temperature = self._drift(
            self._temperature, self._temp_range, max_delta=0.5
        )
        self._humidity = self._drift(
            self._humidity, self._humidity_range, max_delta=1.0
        )
        self._soil_moisture = self._drift(
            self._soil_moisture, self._soil_range, max_delta=0.8
        )

        return SensorReading(
            temperature=round(self._temperature, 2),
            humidity=round(self._humidity, 2),
            soil_moisture=round(self._soil_moisture, 2),
        )

    async def cleanup(self) -> None:
        """No hardware resources to release."""
        pass

    @staticmethod
    def _drift(value: float, bounds: tuple[float, float], max_delta: float) -> float:
        """Apply normally-distributed drift, clamped to bounds."""
        delta = random.gauss(0, max_delta)
        return max(bounds[0], min(bounds[1], value + delta))
