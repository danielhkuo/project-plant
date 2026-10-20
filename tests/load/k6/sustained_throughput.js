// k6 open-loop ingestion test — constant 500 RPS arrival rate (default 30m soak).
//
// This is the SLO gate (p99 < 50ms @ 500 RPS). constant-arrival-rate starts
// RATE requests/s regardless of how fast the server answers, so the load is
// fixed and latency is what's measured.
//
// Run:  k6 run tests/load/k6/sustained_throughput.js            (30m soak)
// CI:   RATE=500 DURATION=5m STRICT=1 k6 run ...                 (nightly gate)
//
// Env overrides: API_URL, API_KEY, RATE, DURATION.
import http from "k6/http";
import { check } from "k6";
import { thresholds, telemetryPayload, authHeaders } from "./lib.js";

const API_URL = __ENV.API_URL || "http://localhost:8080";
const API_KEY = __ENV.API_KEY || "dev-key-001";
const RATE = parseInt(__ENV.RATE || "500", 10);

export const options = {
  scenarios: {
    soak: {
      executor: "constant-arrival-rate",
      rate: RATE,
      timeUnit: "1s",
      duration: __ENV.DURATION || "30m",
      preAllocatedVUs: Math.max(RATE, 100),
      maxVUs: RATE * 4,
    },
  },
  thresholds: thresholds({
    http_req_duration: ["p(99)<50"],
    http_req_failed: ["rate<0.001"],
  }),
};

export default function () {
  const res = http.post(
    `${API_URL}/api/v1/telemetry`,
    JSON.stringify(telemetryPayload()),
    { headers: authHeaders(API_KEY), tags: { name: "ingest" } },
  );
  check(res, { "status is 202": (r) => r.status === 202 });
}
