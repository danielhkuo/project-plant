// k6 soak test — constant 500 RPS for 30 minutes.
//
// Run:  k6 run tests/load/k6/sustained_throughput.js
// CI:   STRICT=1 k6 run ...   (nightly only — this is a 30m test)
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
