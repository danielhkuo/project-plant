"""Base sensor adapter interface for the Device SDK."""

from abc import ABC, abstractmethod
from typing import TypedDict


class SensorReading(TypedDict):
    """A single sensor reading with temperature, humidity, and soil moisture."""

    temperature: float
    humidity: float
    soil_moisture: float


class SensorAdapter(ABC):
    """Abstract base class for sensor adapters.

    Adapter authors implement this interface to integrate hardware sensors
    with the Device SDK. The SDK handles transport, auth, retry, and batching.
    """

    @abstractmethod
    async def initialize(self) -> None:
        """One-time setup (GPIO init, I2C bus open, calibration)."""
        ...

    @abstractmethod
    async def read(self) -> SensorReading:
        """Read current sensor values. Called at ~1Hz by Device."""
        ...

    @abstractmethod
    async def cleanup(self) -> None:
        """Release hardware resources."""
        ...
