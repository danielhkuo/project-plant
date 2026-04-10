# Device Adapter Specification

**Version:** 1.0
**Status:** Stable

---

## Overview

A **Device Adapter** is a Python class that reads sensor data from a specific hardware platform. The adapter implements one interface — `SensorAdapter` — and the Device SDK handles everything else: networking, authentication, retries, batching, timestamps, and error recovery.

Adding support for new hardware means writing one class with three methods. No networking code, no JSON serialization, no auth logic.

---

## Interface Reference

```python
from abc import ABC, abstractmethod
from typing import TypedDict


class SensorReading(TypedDict):
    temperature: float    # Celsius, range [-40, 80]
    humidity: float       # Relative humidity %, range [0, 100]
    soil_moisture: float  # Soil moisture %, range [0, 100]


class SensorAdapter(ABC):

    @abstractmethod
    async def initialize(self) -> None:
        """One-time setup: GPIO init, I2C bus open, calibration.

        Called once before the first read(). If initialization fails,
        the Device will not start.
        """
        ...

    @abstractmethod
    async def read(self) -> SensorReading:
        """Read current sensor values.

        Called at ~1Hz by the Device. Must return a SensorReading with
        all three fields populated and within the valid ranges.

        If this method raises an exception, the Device logs the error
        and retries on the next tick. It does not crash.
        """
        ...

    @abstractmethod
    async def cleanup(self) -> None:
        """Release hardware resources: close I2C bus, release GPIO pins.

        Called once when the Device stops, even if read() has been
        raising errors. Must not raise.
        """
        ...
```

---

## Lifecycle

```
initialize()  ─── called once at startup
     │
     ▼
read()  ◄──── called at ~1Hz in a loop
     │
     ▼
cleanup()  ─── called once at shutdown
```

1. **`initialize()`** — The Device calls this before the first `read()`. Use it for GPIO setup, I2C bus initialization, ADC calibration, or any one-time hardware configuration.

2. **`read()`** — Called approximately once per second. Returns a `SensorReading` dict with `temperature`, `humidity`, and `soil_moisture`. If your hardware read takes time (e.g., DHT22 needs ~2s), that's fine — the method is async.

3. **`cleanup()`** — Called once when the Device shuts down. Release any hardware resources (GPIO pins, file descriptors, I2C buses). This is called even if `read()` has been failing.

---

## Payload Format

Every reading your adapter produces is wrapped by the Device into a telemetry payload conforming to `schemas/telemetry_event.json`:

```json
{
  "device_id": "dev-001",
  "timestamp": "2026-04-09T12:34:56.789Z",
  "temperature": 23.5,
  "humidity": 62.3,
  "soil_moisture": 45.1
}
```

**You provide:** `temperature`, `humidity`, `soil_moisture`
**The Device SDK adds:** `device_id`, `timestamp`

### Field Ranges

| Field | Type | Min | Max | Unit |
|-------|------|-----|-----|------|
| `temperature` | float | -40 | 80 | Celsius |
| `humidity` | float | 0 | 100 | % RH |
| `soil_moisture` | float | 0 | 100 | % |

Values outside these ranges will fail validation at the ingestion API.

---

## Auth Flow

Devices authenticate with the ingestion API using API keys:

1. Each device is assigned an API key during registration
2. The API key is passed to the `HTTPTransport` (part of the SDK), not to the adapter
3. The transport adds the `X-API-Key` header to every HTTP request
4. **Adapter authors never touch authentication**

---

## Transport

The Device SDK handles all networking. Your adapter only returns `SensorReading` dicts.

**What the SDK does for you:**
- HTTP POST to the ingestion API
- `X-API-Key` authentication header
- Automatic retry with exponential backoff (3 attempts)
- JSON serialization of payloads
- Timestamp injection (UTC ISO 8601)
- Error logging and recovery

**What your adapter does:**
- Read hardware sensors
- Return a `SensorReading` dict

---

## Example Adapters

### Minimal Example (15 lines — for testing)

```python
from simulators.adapters import SensorAdapter, SensorReading


class StaticAdapter(SensorAdapter):
    """Returns fixed values. Useful for testing."""

    async def initialize(self) -> None:
        pass

    async def read(self) -> SensorReading:
        return SensorReading(temperature=22.0, humidity=55.0, soil_moisture=40.0)

    async def cleanup(self) -> None:
        pass
```

### Raspberry Pi + DHT22 (pseudocode)

```python
import adafruit_dht
import board
from simulators.adapters import SensorAdapter, SensorReading


class RPiDHT22Adapter(SensorAdapter):
    """Reads temperature and humidity from a DHT22 sensor on a Raspberry Pi."""

    def __init__(self, pin=board.D4):
        self._pin = pin
        self._sensor = None

    async def initialize(self) -> None:
        self._sensor = adafruit_dht.DHT22(self._pin)

    async def read(self) -> SensorReading:
        return SensorReading(
            temperature=self._sensor.temperature,
            humidity=self._sensor.humidity,
            soil_moisture=0.0,  # DHT22 doesn't measure soil — pair with a soil sensor
        )

    async def cleanup(self) -> None:
        if self._sensor:
            self._sensor.exit()
```

### ESP32 + Capacitive Soil Sensor (pseudocode)

```python
from machine import ADC, Pin
from simulators.adapters import SensorAdapter, SensorReading


class ESP32SoilAdapter(SensorAdapter):
    """Reads soil moisture from a capacitive sensor via ESP32 ADC."""

    def __init__(self, adc_pin: int = 34):
        self._adc_pin = adc_pin
        self._adc = None

    async def initialize(self) -> None:
        self._adc = ADC(Pin(self._adc_pin))
        self._adc.atten(ADC.ATTN_11DB)  # Full range: 0-3.3V

    async def read(self) -> SensorReading:
        raw = self._adc.read()
        # Map ADC range (0-4095) to soil moisture percentage
        # Dry air ~3500, water ~1500
        moisture = max(0, min(100, (3500 - raw) / 20))
        return SensorReading(
            temperature=0.0,   # Pair with a separate temp sensor
            humidity=0.0,      # Pair with a separate humidity sensor
            soil_moisture=round(moisture, 2),
        )

    async def cleanup(self) -> None:
        pass  # ADC doesn't need explicit cleanup
```

---

## Testing Your Adapter

Run the built-in adapter test suite against your implementation:

```python
# tests/unit/test_my_adapter.py
import pytest
from simulators.adapters import SensorAdapter, SensorReading
from simulators.device import Device
from my_adapters import MyCustomAdapter


class TestMyAdapter:
    async def test_implements_interface(self):
        adapter = MyCustomAdapter()
        assert isinstance(adapter, SensorAdapter)

    async def test_read_returns_valid_reading(self):
        adapter = MyCustomAdapter()
        await adapter.initialize()
        reading = await adapter.read()
        assert set(reading.keys()) == {"temperature", "humidity", "soil_moisture"}
        assert -40 <= reading["temperature"] <= 80
        assert 0 <= reading["humidity"] <= 100
        assert 0 <= reading["soil_moisture"] <= 100
        await adapter.cleanup()

    async def test_works_with_device(self):
        device = Device("test-001", MyCustomAdapter())
        async for payload in device.start():
            assert payload["device_id"] == "test-001"
            assert "timestamp" in payload
            await device.stop()
            break
```

Run with: `pytest tests/unit/test_my_adapter.py -v`
