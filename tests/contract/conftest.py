import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator


SCHEMA_DIR = Path(__file__).resolve().parent.parent.parent / "schemas"


@pytest.fixture
def telemetry_schema() -> dict:
    """Load the telemetry event JSON schema."""
    schema_path = SCHEMA_DIR / "telemetry_event.json"
    with open(schema_path) as f:
        return json.load(f)


@pytest.fixture
def telemetry_validator(telemetry_schema: dict) -> Draft202012Validator:
    """Create a JSON Schema validator for telemetry events."""
    Draft202012Validator.check_schema(telemetry_schema)
    return Draft202012Validator(telemetry_schema)


@pytest.fixture
def valid_payload() -> dict:
    """A known-good telemetry payload."""
    return {
        "device_id": "dev-001",
        "timestamp": "2026-04-09T12:00:00Z",
        "temperature": 25.3,
        "humidity": 60.0,
        "soil_moisture": 45.5,
    }
