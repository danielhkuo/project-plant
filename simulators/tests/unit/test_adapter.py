"""Tests for SensorAdapter interface and SimulatedSensorAdapter implementation."""

import inspect


from simulators.adapters import SensorAdapter, SensorReading, SimulatedSensorAdapter
from simulators.device import Device


class TestSensorAdapterInterface:
    """Test that SimulatedSensorAdapter correctly implements the SensorAdapter ABC."""

    def test_simulated_adapter_implements_interface(self):
        """SimulatedSensorAdapter is a valid SensorAdapter subclass."""
        adapter = SimulatedSensorAdapter()
        assert isinstance(adapter, SensorAdapter)
        assert not inspect.isabstract(adapter.__class__)  # concrete, not abstract

    async def test_adapter_read_returns_required_keys(self):
        """read() returns dict with exactly temperature, humidity, soil_moisture."""
        adapter = SimulatedSensorAdapter()
        await adapter.initialize()
        reading = await adapter.read()
        assert set(reading.keys()) == {"temperature", "humidity", "soil_moisture"}
        await adapter.cleanup()

    async def test_adapter_read_values_in_range(self):
        """All returned values are within schema-defined bounds."""
        adapter = SimulatedSensorAdapter()
        await adapter.initialize()
        for _ in range(50):
            reading = await adapter.read()
            assert -40 <= reading["temperature"] <= 80
            assert 0 <= reading["humidity"] <= 100
            assert 0 <= reading["soil_moisture"] <= 100
        await adapter.cleanup()

    async def test_adapter_is_async(self):
        """read() is a coroutine (supports I2C/SPI delays on real hardware)."""
        adapter = SimulatedSensorAdapter()
        assert inspect.iscoroutinefunction(adapter.read)
        assert inspect.iscoroutinefunction(adapter.initialize)
        assert inspect.iscoroutinefunction(adapter.cleanup)

    async def test_custom_adapter_works_with_device(self):
        """A hand-written test adapter plugs into Device and produces valid payloads."""

        class FixedAdapter(SensorAdapter):
            async def initialize(self) -> None:
                pass

            async def read(self) -> SensorReading:
                return SensorReading(temperature=22.0, humidity=55.0, soil_moisture=40.0)

            async def cleanup(self) -> None:
                pass

        device = Device("test-fixed-001", FixedAdapter())
        payloads = []
        async for payload in device.start():
            payloads.append(payload)
            if len(payloads) >= 2:
                await device.stop()
                break

        for p in payloads:
            assert p["temperature"] == 22.0
            assert p["humidity"] == 55.0
            assert p["soil_moisture"] == 40.0
            assert p["device_id"] == "test-fixed-001"

    async def test_adapter_read_error_handling(self):
        """If read() raises, Device logs the error and retries on next tick (doesn't crash)."""

        call_count = 0

        class FlakyAdapter(SensorAdapter):
            async def initialize(self) -> None:
                pass

            async def read(self) -> SensorReading:
                nonlocal call_count
                call_count += 1
                if call_count <= 2:
                    raise RuntimeError("Sensor read failed")
                return SensorReading(temperature=20.0, humidity=50.0, soil_moisture=30.0)

            async def cleanup(self) -> None:
                pass

        device = Device("test-flaky-001", FlakyAdapter())
        payloads = []
        async for payload in device.start():
            payloads.append(payload)
            if len(payloads) >= 1:
                await device.stop()
                break

        # Device survived the errors and eventually produced a valid payload
        assert len(payloads) >= 1
        assert payloads[0]["temperature"] == 20.0
        assert call_count >= 3  # At least 2 failures + 1 success
