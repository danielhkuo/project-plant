import { http, HttpResponse } from "msw";
import {
  demoDevices,
  demoAlerts,
  demoStats,
  generateHistory,
} from "./fixtures";
import type { Alert, DeviceWithStatus } from "@/lib/types";

// Mutable copies so resolve actions persist within a session
let devices: DeviceWithStatus[] = structuredClone(demoDevices);
let alerts: Alert[] = structuredClone(demoAlerts);

export const handlers = [
  http.get("/api/v1/devices", () => HttpResponse.json(devices)),

  http.get("/api/v1/devices/:id", ({ params }) => {
    const device = devices.find((d) => d.device_id === params.id);
    if (!device) {
      return HttpResponse.json({ error: "device not found" }, { status: 404 });
    }
    return HttpResponse.json(device);
  }),

  http.get("/api/v1/devices/:id/history", ({ params, request }) => {
    const url = new URL(request.url);
    const from = url.searchParams.get("from");
    const to = url.searchParams.get("to");
    let hours = 1;
    if (from && to) {
      hours = Math.max(
        1,
        Math.round(
          (new Date(to).getTime() - new Date(from).getTime()) / (60 * 60_000)
        )
      );
    }
    const device = devices.find((d) => d.device_id === params.id);
    const history = generateHistory(
      String(params.id),
      hours,
      device?.latest.temperature ?? 22,
      device?.latest.humidity ?? 65,
      device?.latest.soil_moisture ?? 45
    );
    return HttpResponse.json(history);
  }),

  http.get("/api/v1/alerts", ({ request }) => {
    const url = new URL(request.url);
    const severity = url.searchParams.get("severity");
    const status = url.searchParams.get("status");
    const deviceId = url.searchParams.get("device_id");

    let filtered = alerts;
    if (severity) filtered = filtered.filter((a) => a.severity === severity);
    if (deviceId) filtered = filtered.filter((a) => a.device_id === deviceId);
    if (status === "active")
      filtered = filtered.filter((a) => a.resolved_at === null);
    if (status === "resolved")
      filtered = filtered.filter((a) => a.resolved_at !== null);

    return HttpResponse.json(filtered);
  }),

  http.post("/api/v1/alerts/:id/resolve", ({ params }) => {
    alerts = alerts.map((a) =>
      a.alert_id === params.id
        ? { ...a, resolved_at: new Date().toISOString() }
        : a
    );
    return HttpResponse.json({ alert_id: params.id, status: "resolved" });
  }),

  http.get("/api/v1/stats", () => {
    const stats = {
      ...demoStats,
      device_count: devices.length,
      active_alerts: alerts.filter((a) => a.resolved_at === null).length,
    };
    return HttpResponse.json(stats);
  }),

  http.get("/health", () => HttpResponse.json({ status: "ok" })),
];

// Simulate live telemetry updates (called by the demo tick)
export function tickDemoReadings() {
  devices = devices.map((d) => ({
    ...d,
    latest: {
      ...d.latest,
      timestamp: new Date().toISOString(),
      temperature: d.latest.temperature + (Math.random() - 0.5) * 0.8,
      humidity: Math.max(
        0,
        Math.min(100, d.latest.humidity + (Math.random() - 0.5) * 1.2)
      ),
      soil_moisture: Math.max(
        0,
        Math.min(100, d.latest.soil_moisture + (Math.random() - 0.5) * 0.5)
      ),
    },
  }));
}
