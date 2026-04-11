import { http, HttpResponse } from "msw";
import { mockDevices, mockAlerts, mockHistory, mockStats } from "./fixtures";

export const handlers = [
  http.get("/api/v1/devices", () => {
    return HttpResponse.json(mockDevices);
  }),

  http.get("/api/v1/devices/:id", ({ params }) => {
    const device = mockDevices.find((d) => d.device_id === params.id);
    if (!device) {
      return HttpResponse.json({ error: "device not found" }, { status: 404 });
    }
    return HttpResponse.json(device);
  }),

  http.get("/api/v1/devices/:id/history", () => {
    return HttpResponse.json(mockHistory);
  }),

  http.get("/api/v1/alerts", () => {
    return HttpResponse.json(mockAlerts);
  }),

  http.post("/api/v1/alerts/:id/resolve", ({ params }) => {
    return HttpResponse.json({
      alert_id: params.id,
      status: "resolved",
    });
  }),

  http.get("/api/v1/stats", () => {
    return HttpResponse.json(mockStats);
  }),

  http.get("/health", () => {
    return HttpResponse.json({ status: "ok" });
  }),
];
