"""Unit tests for the HTTP transport (mocked httpx, no live server)."""

import httpx
import pytest
import respx

from simulators.transport import HTTPTransport

BASE_URL = "http://ingestion.test"


def _reading(device_id: str = "dev-000") -> dict:
    return {
        "device_id": device_id,
        "timestamp": "2026-06-08T00:00:00+00:00",
        "temperature": 22.5,
        "humidity": 55.0,
        "soil_moisture": 40.0,
    }


@respx.mock
async def test_http_transport_sends_post():
    route = respx.post(f"{BASE_URL}/api/v1/telemetry").mock(
        return_value=httpx.Response(202)
    )

    async with HTTPTransport(BASE_URL, "dev-key-001") as transport:
        await transport.send(_reading())

    assert route.called
    sent = route.calls.last.request
    assert sent.method == "POST"
    import json

    assert json.loads(sent.content) == _reading()


@respx.mock
async def test_http_transport_includes_api_key():
    route = respx.post(f"{BASE_URL}/api/v1/telemetry").mock(
        return_value=httpx.Response(202)
    )

    async with HTTPTransport(BASE_URL, "secret-key") as transport:
        await transport.send(_reading())

    assert route.calls.last.request.headers["X-API-Key"] == "secret-key"


@respx.mock
async def test_http_transport_retries_on_5xx():
    route = respx.post(f"{BASE_URL}/api/v1/telemetry").mock(
        return_value=httpx.Response(503)
    )

    async with HTTPTransport(BASE_URL, "k", retries=3, backoff_base=0.0) as transport:
        with pytest.raises(httpx.HTTPStatusError):
            await transport.send(_reading())

    # initial attempt + 3 retries
    assert route.call_count == 4


@respx.mock
async def test_http_transport_no_retry_on_4xx():
    route = respx.post(f"{BASE_URL}/api/v1/telemetry").mock(
        return_value=httpx.Response(400)
    )

    async with HTTPTransport(BASE_URL, "k", retries=3, backoff_base=0.0) as transport:
        with pytest.raises(httpx.HTTPStatusError):
            await transport.send(_reading())

    assert route.call_count == 1


@respx.mock
async def test_http_transport_timeout():
    route = respx.post(f"{BASE_URL}/api/v1/telemetry").mock(
        side_effect=httpx.TimeoutException("timed out")
    )

    async with HTTPTransport(BASE_URL, "k", retries=3, backoff_base=0.0) as transport:
        with pytest.raises(httpx.TimeoutException):
            await transport.send(_reading())

    # timeouts are not retried — exactly one attempt, no hang
    assert route.call_count == 1


@respx.mock
async def test_http_transport_batch_mode():
    route = respx.post(f"{BASE_URL}/api/v1/telemetry/batch").mock(
        return_value=httpx.Response(202)
    )
    batch = [_reading(f"dev-{i:03d}") for i in range(10)]

    async with HTTPTransport(BASE_URL, "k", batch_size=10) as transport:
        await transport.send_batch(batch)

    assert route.call_count == 1
    import json

    assert json.loads(route.calls.last.request.content) == batch
