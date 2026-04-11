import type {
  DeviceWithStatus,
  TelemetryEvent,
  Alert,
  DeviceStats,
} from "./types";

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  getDevices: () => fetchAPI<DeviceWithStatus[]>("/api/v1/devices"),

  getDevice: (id: string) =>
    fetchAPI<DeviceWithStatus>(`/api/v1/devices/${encodeURIComponent(id)}`),

  getDeviceHistory: (
    id: string,
    params?: { from?: string; to?: string; limit?: number; offset?: number }
  ) => {
    const search = new URLSearchParams();
    if (params?.from) search.set("from", params.from);
    if (params?.to) search.set("to", params.to);
    if (params?.limit) search.set("limit", String(params.limit));
    if (params?.offset) search.set("offset", String(params.offset));
    const qs = search.toString();
    return fetchAPI<TelemetryEvent[]>(
      `/api/v1/devices/${encodeURIComponent(id)}/history${qs ? `?${qs}` : ""}`
    );
  },

  getAlerts: (params?: {
    severity?: string;
    status?: string;
    device_id?: string;
  }) => {
    const search = new URLSearchParams();
    if (params?.severity) search.set("severity", params.severity);
    if (params?.status) search.set("status", params.status);
    if (params?.device_id) search.set("device_id", params.device_id);
    const qs = search.toString();
    return fetchAPI<Alert[]>(`/api/v1/alerts${qs ? `?${qs}` : ""}`);
  },

  resolveAlert: (id: string) =>
    fetchAPI<{ alert_id: string; status: string }>(
      `/api/v1/alerts/${encodeURIComponent(id)}/resolve`,
      { method: "POST" }
    ),

  getStats: () => fetchAPI<DeviceStats>("/api/v1/stats"),
};
