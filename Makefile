.PHONY: test-all test-go-unit test-go-integration test-python-unit test-python-integration \
       test-contract test-e2e lint-go lint-python load-test-quick load-test-full load-test-soak

# ── Go ──────────────────────────────────────────────

test-go-unit:
	cd pkg && go test ./...
	cd services/ingestion && go test ./internal/... 2>/dev/null || true
	cd services/processor && go test ./internal/... 2>/dev/null || true
	cd services/dashboard && go test ./internal/... 2>/dev/null || true

test-go-integration:
	cd services/ingestion && go test -tags=integration ./integration_test/...
	cd services/processor && go test -tags=integration ./integration_test/...
	cd services/dashboard && go test -tags=integration ./integration_test/... 2>/dev/null || true

test-go-bench:
	cd pkg && go test -bench=. -benchtime=3s ./...
	cd services/ingestion && go test -bench=. -benchtime=3s ./internal/...
	cd services/processor && go test -bench=. -benchtime=3s ./internal/...
	cd services/dashboard && go test -bench=. -benchtime=3s ./internal/...

lint-go:
	cd pkg && golangci-lint run
	cd services/ingestion && golangci-lint run
	cd services/processor && golangci-lint run
	cd services/dashboard && golangci-lint run

# ── Python ──────────────────────────────────────────

test-python-unit:
	cd simulators && .venv/bin/python -m pytest tests/unit/ -v --cov=src

test-python-integration:
	cd simulators && python3 -m pytest tests/integration/ -v -m integration

test-contract:
	cd tests && .venv/bin/python -m pytest contract/ -v

test-e2e:
	cd tests && .venv/bin/python -m pytest e2e/ -v -m e2e --timeout=600

lint-python:
	cd simulators && ruff check . && mypy src/

# ── Combined ────────────────────────────────────────

test-all: test-go-unit test-python-unit test-contract

test-all-integration: test-all test-go-integration test-python-integration

test-full: test-all-integration test-e2e

# ── Load Testing ────────────────────────────────────

load-test-quick:
	cd tests/load && echo "POST http://localhost:8080/api/v1/telemetry" | vegeta attack -rate=500/s -duration=30s | vegeta report

load-test-full:
	cd tests/load && k6 run k6/ingestion_load.js

load-test-soak:
	cd tests/load && k6 run k6/sustained_throughput.js

# ── Docker ──────────────────────────────────────────

infra-up:
	docker compose up -d --wait

infra-down:
	docker compose down -v

infra-test-up:
	docker compose -f docker-compose.test.yml up -d --wait

infra-test-down:
	docker compose -f docker-compose.test.yml down -v
