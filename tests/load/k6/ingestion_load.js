// k6 closed-loop stress test — ramp 0->500 VUs over 2m, sustain 5m.
//
// Each VU fires its next request as soon as the last one returns (no sleep),
// so throughput is whatever the server can absorb (~6,200 RPS on CI), not 500.
// That makes it a capacity/stress run, NOT the SLO gate: it has no thresholds.
// The p99 < 50ms @ 500 RPS gate is sustained_throughput.js.
//
// Run:  k6 run tests/load/k6/ingestion_load.js   (or: make load-test-stress)
//
// Env overrides: API_URL, API_KEY, RAMP_TARGET, RAMP_UP, SUSTAIN.
import http from "k6/http";
import { check } from "k6";
import { telemetryPayload, authHeaders } from "./lib.js";

const API_URL = __ENV.API_URL || "http://localhost:8080";
const API_KEY = __ENV.API_KEY || "dev-key-001";
const TARGET = parseInt(__ENV.RAMP_TARGET || "500", 10);

export const options = {
  scenarios: {
    ingestion: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: __ENV.RAMP_UP || "2m", target: TARGET },
        { duration: __ENV.SUSTAIN || "5m", target: TARGET },
        { duration: "10s", target: 0 },
      ],
      gracefulStop: "10s",
    },
  },
};

export default function () {
  const res = http.post(
    `${API_URL}/api/v1/telemetry`,
    JSON.stringify(telemetryPayload()),
    { headers: authHeaders(API_KEY), tags: { name: "ingest" } },
  );
  check(res, { "status is 202": (r) => r.status === 202 });
}
