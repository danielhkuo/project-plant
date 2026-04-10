"""Tests for environmental profiles."""

import pytest

from simulators.adapters import SimulatedSensorAdapter
from simulators.profiles import Profile, get_profile, PROFILES


class TestProfiles:
    """Test environmental profile definitions and constraints."""

    def test_tropical_profile_ranges(self):
        p = get_profile("tropical")
        assert p.temp_range == (20, 35)
        assert p.humidity_range == (60, 95)
        assert p.soil_range == (40, 80)

    def test_arid_profile_ranges(self):
        p = get_profile("arid")
        assert p.temp_range == (25, 50)
        assert p.humidity_range == (5, 30)
        assert p.soil_range == (5, 25)

    def test_temperate_profile_ranges(self):
        p = get_profile("temperate")
        assert p.temp_range == (15, 28)
        assert p.humidity_range == (40, 70)
        assert p.soil_range == (30, 60)

    def test_dying_profile_ranges(self):
        p = get_profile("dying")
        assert p.temp_range == (30, 45)
        assert p.humidity_range == (10, 25)
        assert p.soil_range == (2, 15)

    def test_default_profile_exists(self):
        p = get_profile("default")
        assert p is not None
        assert isinstance(p, Profile)

    def test_unknown_profile_raises(self):
        with pytest.raises(ValueError, match="nonexistent"):
            get_profile("nonexistent")

    async def test_profile_constrains_1000_readings(self):
        """Generate 1000 readings with tropical profile, all within bounds."""
        p = get_profile("tropical")
        adapter = SimulatedSensorAdapter(
            temp_range=p.temp_range,
            humidity_range=p.humidity_range,
            soil_range=p.soil_range,
        )
        await adapter.initialize()

        for _ in range(1000):
            reading = await adapter.read()
            assert p.temp_range[0] <= reading["temperature"] <= p.temp_range[1], (
                f"temp {reading['temperature']} outside {p.temp_range}"
            )
            assert p.humidity_range[0] <= reading["humidity"] <= p.humidity_range[1], (
                f"humidity {reading['humidity']} outside {p.humidity_range}"
            )
            assert p.soil_range[0] <= reading["soil_moisture"] <= p.soil_range[1], (
                f"soil {reading['soil_moisture']} outside {p.soil_range}"
            )

    def test_profile_initial_values(self):
        """Profile provides sensible starting midpoints, not random."""
        p = get_profile("tropical")
        expected_temp = (20 + 35) / 2
        expected_humidity = (60 + 95) / 2
        expected_soil = (40 + 80) / 2
        assert p.initial_values["temperature"] == expected_temp
        assert p.initial_values["humidity"] == expected_humidity
        assert p.initial_values["soil_moisture"] == expected_soil
