"""Contract tests: validate telemetry payloads against the shared JSON schema.

These tests are the single source of truth for cross-service payload compatibility.
Both Go and Python services must produce payloads that pass these validations.
"""

import pytest
from jsonschema import Draft202012Validator, ValidationError


class TestValidPayloads:
    """Known-good payloads must be accepted."""

    def test_valid_payload_passes(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        telemetry_validator.validate(valid_payload)

    def test_integer_values_accepted(self, telemetry_validator: Draft202012Validator):
        """Integer values are valid numbers."""
        payload = {
            "device_id": "dev-002",
            "timestamp": "2026-04-09T12:00:00Z",
            "temperature": 25,
            "humidity": 60,
            "soil_moisture": 45,
        }
        telemetry_validator.validate(payload)

    def test_boundary_values_min(self, telemetry_validator: Draft202012Validator):
        """Minimum boundary values are accepted."""
        payload = {
            "device_id": "x",
            "timestamp": "2026-01-01T00:00:00+00:00",
            "temperature": -40,
            "humidity": 0,
            "soil_moisture": 0,
        }
        telemetry_validator.validate(payload)

    def test_boundary_values_max(self, telemetry_validator: Draft202012Validator):
        """Maximum boundary values are accepted."""
        payload = {
            "device_id": "dev-999",
            "timestamp": "2026-12-31T23:59:59Z",
            "temperature": 80,
            "humidity": 100,
            "soil_moisture": 100,
        }
        telemetry_validator.validate(payload)


class TestMissingRequiredFields:
    """Payloads missing required fields must be rejected."""

    @pytest.mark.parametrize("missing_field", [
        "device_id",
        "timestamp",
        "temperature",
        "humidity",
        "soil_moisture",
    ])
    def test_missing_required_field(
        self,
        telemetry_validator: Draft202012Validator,
        valid_payload: dict,
        missing_field: str,
    ):
        payload = {k: v for k, v in valid_payload.items() if k != missing_field}
        with pytest.raises(ValidationError, match=f"'{missing_field}' is a required property"):
            telemetry_validator.validate(payload)


class TestInvalidTypes:
    """Payloads with wrong types must be rejected."""

    def test_device_id_not_string(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["device_id"] = 123
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_temperature_not_number(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["temperature"] = "hot"
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_humidity_not_number(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["humidity"] = True
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_soil_moisture_not_number(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["soil_moisture"] = None
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)


class TestOutOfRangeValues:
    """Payloads with out-of-range values must be rejected."""

    def test_temperature_too_low(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["temperature"] = -41
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_temperature_too_high(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["temperature"] = 81
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_humidity_below_zero(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["humidity"] = -1
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_humidity_above_100(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["humidity"] = 101
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_soil_moisture_below_zero(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["soil_moisture"] = -0.1
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_soil_moisture_above_100(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["soil_moisture"] = 100.1
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)


class TestConstraintViolations:
    """Other constraint violations must be rejected."""

    def test_empty_device_id(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["device_id"] = ""
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_extra_fields_rejected(self, telemetry_validator: Draft202012Validator, valid_payload: dict):
        valid_payload["extra_field"] = "should not be here"
        with pytest.raises(ValidationError):
            telemetry_validator.validate(valid_payload)

    def test_empty_object_rejected(self, telemetry_validator: Draft202012Validator):
        with pytest.raises(ValidationError):
            telemetry_validator.validate({})

    def test_null_rejected(self, telemetry_validator: Draft202012Validator):
        with pytest.raises(ValidationError):
            telemetry_validator.validate(None)
