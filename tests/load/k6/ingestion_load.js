// k6 ingestion load test — ramp 0->500 VUs over 2m, sustain 5m.
//
// Run:  k6 run tests/load/k6/ingestion_load.js
// CI:   STRICT=1 k6 run ...   (thresholds hard-fail; locally they're informational)
//
// Env overrides: API_URL, API_KEY, RAMP_TARGET, RAMP_UP, SUSTAIN.
import http from "k6/http";
import { check } from "k6";
import { thresholds, telemetryPayload, authHeaders } from "./lib.js";

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
  // p99 < 50ms, error rate < 0.1%, throughput > 500 rps — gated only when STRICT=1.
  thresholds: thresholds({
    http_req_duration: ["p(99)<50"],
    http_req_failed: ["rate<0.001"],
    http_reqs: ["rate>500"],
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
