"""CLI entry point: run a fleet of simulated devices against the ingestion API.

Example:
    python -m simulators --count 10 --profile tropical \
        --api-url http://localhost:8080 --api-key dev-key-001 --duration 10
"""

import argparse
import asyncio
import logging

from .fleet import Fleet
from .transport import HTTPTransport


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="simulators", description=__doc__)
    parser.add_argument("--count", type=int, default=1, help="number of devices")
    parser.add_argument(
        "--profile",
        action="append",
        dest="profiles",
        help="environmental profile (repeatable; round-robin across devices)",
    )
    parser.add_argument(
        "--api-url", default="http://localhost:8080", help="ingestion API base URL"
    )
    parser.add_argument("--api-key", default="dev-key-001", help="ingestion API key")
    parser.add_argument(
        "--duration",
        type=float,
        default=None,
        help="run for N seconds, then stop (default: run until interrupted)",
    )
    parser.add_argument("--timeout", type=float, default=5.0, help="per-request timeout (s)")
    parser.add_argument("--retries", type=int, default=3, help="retries on 5xx")
    return parser.parse_args(argv)


async def run(args: argparse.Namespace) -> None:
    transport = HTTPTransport(
        base_url=args.api_url,
        api_key=args.api_key,
        timeout=args.timeout,
        retries=args.retries,
    )

    async def on_reading(reading: dict) -> None:
        try:
            await transport.send(reading)
        except Exception:
            logging.getLogger(__name__).exception(
                "failed to send reading for %s", reading.get("device_id")
            )

    fleet = Fleet(count=args.count, profiles=args.profiles, on_reading=on_reading)

    logging.info(
        "starting fleet: count=%d profiles=%s -> %s",
        args.count,
        args.profiles or ["default"],
        args.api_url,
    )
    try:
        await fleet.start(duration=args.duration)
    finally:
        await transport.aclose()


def main(argv: list[str] | None = None) -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    args = parse_args(argv)
    try:
        asyncio.run(run(args))
    except KeyboardInterrupt:
        logging.info("interrupted, shutting down")


if __name__ == "__main__":
    main()
