"""HTTP transport that ships device telemetry to the ingestion API.

The transport plugs into ``Fleet`` via its ``on_reading`` callback: each
reading dict yielded by a device is POSTed to the ingestion service. Server
errors (5xx) are retried with exponential backoff; client errors (4xx) and
timeouts are surfaced immediately without retry.
"""

import asyncio
import logging

import httpx

logger = logging.getLogger(__name__)

TELEMETRY_PATH = "/api/v1/telemetry"
BATCH_PATH = "/api/v1/telemetry/batch"


class HTTPTransport:
    """Async HTTP client for posting telemetry to the ingestion API.

    Args:
        base_url: Ingestion API base URL, e.g. ``http://localhost:8080``.
        api_key: Value sent in the ``X-API-Key`` header.
        timeout: Per-request timeout in seconds.
        retries: Number of retries *after* the initial attempt on 5xx errors.
        batch_size: Default batch size hint for callers that buffer readings.
        backoff_base: Base seconds for exponential backoff between retries.
        client: Optional pre-built ``httpx.AsyncClient`` (mainly for tests).
    """

    def __init__(
        self,
        base_url: str,
        api_key: str,
        *,
        timeout: float = 5.0,
        retries: int = 3,
        batch_size: int = 1,
        backoff_base: float = 0.1,
        client: httpx.AsyncClient | None = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._timeout = timeout
        self._retries = retries
        self.batch_size = batch_size
        self._backoff_base = backoff_base
        self._client = client or httpx.AsyncClient(timeout=timeout)
        self._owns_client = client is None

    @property
    def _headers(self) -> dict[str, str]:
        return {"X-API-Key": self._api_key, "Content-Type": "application/json"}

    async def send(self, reading: dict) -> None:
        """POST a single telemetry reading to the ingestion API."""
        await self._post(TELEMETRY_PATH, reading)

    async def send_batch(self, readings: list[dict]) -> None:
        """POST a batch of readings as a single request."""
        await self._post(BATCH_PATH, readings)

    async def _post(self, path: str, payload: dict | list) -> None:
        url = f"{self._base_url}{path}"

        for attempt in range(self._retries + 1):
            try:
                resp = await self._client.post(url, json=payload, headers=self._headers)
            except httpx.TimeoutException:
                # Timeouts are not retried — surface immediately so callers
                # don't hang behind a backoff loop.
                logger.warning("telemetry POST to %s timed out", url)
                raise

            if resp.status_code < 400:
                return

            if 400 <= resp.status_code < 500:
                # Client error — our request is wrong; retrying won't help.
                resp.raise_for_status()

            # 5xx — transient server error; retry with exponential backoff.
            if attempt < self._retries:
                backoff = self._backoff_base * (2**attempt)
                logger.warning(
                    "telemetry POST to %s failed (%d), retry %d/%d in %.2fs",
                    url,
                    resp.status_code,
                    attempt + 1,
                    self._retries,
                    backoff,
                )
                await asyncio.sleep(backoff)
                continue

            resp.raise_for_status()

    async def aclose(self) -> None:
        """Close the underlying client if this transport created it."""
        if self._owns_client:
            await self._client.aclose()

    async def __aenter__(self) -> "HTTPTransport":
        return self

    async def __aexit__(self, *_exc: object) -> None:
        await self.aclose()
