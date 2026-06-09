// k6 dashboard read test — 50 concurrent users polling GET /api/v1/devices
// every ~2s. Reads should stay fast (p99 < 100ms) even while ingestion is under
// write load, so run this alongside background write load, e.g.:
//
//   make run-simulators COUNT=200 DURATION=300 &   # background writes
//   make load-test-dashboard
//
// Env overrides: DASHBOARD_URL, VUS, DURATION.
import http from "k6/http";
import { check, sleep } from "k6";
import { thresholds } from "./lib.js";

const DASHBOARD_URL = __ENV.DASHBOARD_URL || "http://localhost:8081";

export const options = {
  scenarios: {
    readers: {
      executor: "constant-vus",
      vus: parseInt(__ENV.VUS || "50", 10),
      duration: __ENV.DURATION || "3m",
    },
  },
  // Reads should stay fast under write pressure.
  thresholds: thresholds({
    "http_req_duration{name:devices}": ["p(99)<100"],
    http_req_failed: ["rate<0.001"],
  }),
};

export default function () {
  const res = http.get(`${DASHBOARD_URL}/api/v1/devices`, {
    tags: { name: "devices" },
  });
  check(res, { "status is 200": (r) => r.status === 200 });
  sleep(2);
}
