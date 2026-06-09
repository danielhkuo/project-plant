// Shared helpers for the k6 load scripts.

// thresholds() gates the run only when STRICT=1 (CI on Linux). Locally it
// returns {} so k6 still measures and prints p99/throughput in the summary
// without failing the build — see tests/load/README.md for why.
export function thresholds(strict) {
  return __ENV.STRICT === "1" ? strict : {};
}

export function authHeaders(apiKey) {
  return { "Content-Type": "application/json", "X-API-Key": apiKey };
}

// A valid telemetry event with a per-VU device id and current timestamp.
// device id is spread across a fixed fleet so writes hit many keys.
export function telemetryPayload() {
  const deviceNum = (__VU - 1) % 200;
  return {
    device_id: `dev-${String(deviceNum).padStart(3, "0")}`,
    timestamp: new Date().toISOString(),
    temperature: 18 + Math.random() * 12,
    humidity: 40 + Math.random() * 40,
    soil_moisture: 25 + Math.random() * 50,
  };
}
