"""Environmental profiles for simulated sensor devices."""

from dataclasses import dataclass

from .adapters.base import SensorReading


@dataclass(frozen=True)
class Profile:
    """Defines environmental ranges for a sensor simulation."""

    name: str
    temp_range: tuple[float, float]
    humidity_range: tuple[float, float]
    soil_range: tuple[float, float]

    @property
    def initial_values(self) -> SensorReading:
        """Return midpoint values as the starting point."""
        return SensorReading(
            temperature=sum(self.temp_range) / 2,
            humidity=sum(self.humidity_range) / 2,
            soil_moisture=sum(self.soil_range) / 2,
        )


PROFILES: dict[str, Profile] = {
    "tropical": Profile(
        name="tropical",
        temp_range=(20, 35),
        humidity_range=(60, 95),
        soil_range=(40, 80),
    ),
    "arid": Profile(
        name="arid",
        temp_range=(25, 50),
        humidity_range=(5, 30),
        soil_range=(5, 25),
    ),
    "temperate": Profile(
        name="temperate",
        temp_range=(15, 28),
        humidity_range=(40, 70),
        soil_range=(30, 60),
    ),
    "dying": Profile(
        name="dying",
        temp_range=(30, 45),
        humidity_range=(10, 25),
        soil_range=(2, 15),
    ),
}

# "default" is an alias for "temperate"
PROFILES["default"] = PROFILES["temperate"]


def get_profile(name: str) -> Profile:
    """Get a profile by name. Raises ValueError for unknown profiles."""
    if name not in PROFILES:
        raise ValueError(f"Unknown profile: {name!r}. Available: {list(PROFILES.keys())}")
    return PROFILES[name]
