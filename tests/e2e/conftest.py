"""End-to-end test harness: brings up the full Project Plant stack.

Infrastructure (Kafka, Postgres, Redis) runs via the repo docker-compose.yml.
The three Go services (ingestion, processor, dashboard) are built and run as
host subprocesses — per-service Dockerfiles are a later roadmap step (Step 17),
so for E2E we drive them exactly like `make run-*` does.

The `stack` fixture is session-scoped: build once, bring everything up, yield a
Stack handle, then tear everything down.
"""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path

import httpx
import psycopg
import pytest

REPO_ROOT = Path(__file__).resolve().parents[2]
COMPOSE_FILE = REPO_ROOT / "docker-compose.yml"
MIGRATIONS_DIR = REPO_ROOT / "migrations"
PROJECT = "plant_e2e"  # isolated compose project name

# Reuse the real simulator as the load generator (only needs httpx).
sys.path.insert(0, str(REPO_ROOT / "simulators" / "src"))

INGEST_URL = "http://localhost:8080"
DASHBOARD_URL = "http://localhost:8081"
WS_URL = "ws://localhost:8081/api/v1/ws/events"
PG_DSN = "postgresql://plant:plant@localhost:5432/plantdb"
REDIS_URL = "redis://localhost:6379"
API_KEY = "dev-key-001"

# Services: (name, service_dir, build target package, env, log file).
GO_SERVICES = [
    ("ingestion", "services/ingestion", "./cmd/server", {"LISTEN_ADDR": ":8080"}),
    ("processor", "services/processor", "./cmd/processor", {}),
    ("dashboard", "services/dashboard", "./cmd/dashboard", {"LISTEN_ADDR": ":8081"}),
]


@dataclass
class Stack:
    ingest_url: str
    dashboard_url: str
    ws_url: str
    pg_dsn: str
    redis_url: str
    api_key: str
    log_dir: Path
    _procs: dict = field(default_factory=dict)

    def _compose(self, *args: str) -> None:
        subprocess.run(
            ["docker", "compose", "-p", PROJECT, "-f", str(COMPOSE_FILE), *args],
            cwd=REPO_ROOT,
            check=True,
        )

    def stop_service(self, name: str) -> None:
        """Stop an infra container (postgres/redis/kafka) mid-test."""
        self._compose("stop", name)

    def start_service(self, name: str) -> None:
        """Restart a previously stopped infra container."""
        self._compose("start", name)

    def read_logs(self, name: str) -> str:
        path = self.log_dir / f"{name}.log"
        return path.read_text() if path.exists() else ""


def _run(cmd: list[str], cwd: Path | None = None, env: dict | None = None) -> None:
    subprocess.run(cmd, cwd=cwd, env=env, check=True)


def _apply_migrations() -> None:
    sql_files = sorted(MIGRATIONS_DIR.glob("*.up.sql"))
    with psycopg.connect(PG_DSN, autocommit=True) as conn:
        for f in sql_files:
            conn.execute(f.read_text())


def _wait_http_ok(url: str, timeout: float = 40.0) -> None:
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        try:
            r = httpx.get(url, timeout=2.0)
            if r.status_code == 200:
                return
            last = f"status {r.status_code}"
        except Exception as exc:  # noqa: BLE001
            last = str(exc)
        time.sleep(1.0)
    raise RuntimeError(f"service at {url} not healthy within {timeout}s ({last})")


@pytest.fixture(scope="session")
def stack(tmp_path_factory) -> Stack:
    if not COMPOSE_FILE.exists():
        pytest.skip("docker-compose.yml not found")
    if subprocess.run(["docker", "info"], capture_output=True).returncode != 0:
        pytest.skip("docker daemon not available")

    log_dir = tmp_path_factory.mktemp("e2e-logs")
    bin_dir = tmp_path_factory.mktemp("e2e-bin")

    # 1. Build the three Go services.
    built: dict[str, Path] = {}
    for name, rel, pkg, _ in GO_SERVICES:
        out = bin_dir / name
        _run(["go", "build", "-o", str(out), pkg], cwd=REPO_ROOT / rel)
        built[name] = out

    s = Stack(
        ingest_url=INGEST_URL,
        dashboard_url=DASHBOARD_URL,
        ws_url=WS_URL,
        pg_dsn=PG_DSN,
        redis_url=REDIS_URL,
        api_key=API_KEY,
        log_dir=log_dir,
    )

    # 2. Fresh infra + schema.
    subprocess.run(
        ["docker", "compose", "-p", PROJECT, "-f", str(COMPOSE_FILE), "down", "-v"],
        cwd=REPO_ROOT,
        check=False,
    )
    s._compose("up", "-d", "--wait")
    _apply_migrations()

    # 3. Launch services as subprocesses.
    base_env = {
        **os.environ,
        "KAFKA_BROKERS": "localhost:9092",
        "POSTGRES_DSN": PG_DSN + "?sslmode=disable",
        "REDIS_ADDR": "localhost:6379",
    }
    try:
        for name, _rel, _pkg, extra in GO_SERVICES:
            logf = open(log_dir / f"{name}.log", "w")  # noqa: SIM115 (kept open for process lifetime)
            s._procs[name] = (
                subprocess.Popen(
                    [str(built[name])],
                    env={**base_env, **extra},
                    stdout=logf,
                    stderr=subprocess.STDOUT,
                ),
                logf,
            )

        # 4. Wait for HTTP services; give the processor a moment to join Kafka.
        _wait_http_ok(f"{INGEST_URL}/health")
        _wait_http_ok(f"{DASHBOARD_URL}/health")
        time.sleep(3.0)

        yield s
    finally:
        # Teardown: stop services, then infra.
        for name, (proc, logf) in s._procs.items():
            proc.send_signal(signal.SIGINT)
        for name, (proc, logf) in s._procs.items():
            try:
                proc.wait(timeout=8)
            except subprocess.TimeoutExpired:
                proc.kill()
            logf.close()
        subprocess.run(
            ["docker", "compose", "-p", PROJECT, "-f", str(COMPOSE_FILE), "down", "-v"],
            cwd=REPO_ROOT,
            check=False,
        )


@pytest.fixture
def since() -> str:
    """UTC timestamp marking test start, for time-windowed event counts.

    Postgres state is shared across the session, so tests count only rows with
    recorded_at >= this value rather than relying on absolute totals.
    """
    from datetime import datetime, timezone

    return datetime.now(timezone.utc).isoformat()
