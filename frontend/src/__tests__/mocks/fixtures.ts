import type {
  DeviceWithStatus,
  Alert,
  TelemetryEvent,
  DeviceStats,
} from "@/lib/types";

export const mockDevices: DeviceWithStatus[] = [
  {
    device_id: "dev-001",
    status: "normal",
    latest: {
      device_id: "dev-001",
      timestamp: "2026-04-10T12:00:00Z",
      temperature: 22.5,
      humidity: 65.0,
      soil_moisture: 45.0,
    },
  },
  {
    device_id: "dev-002",
    status: "warning",
    latest: {
      device_id: "dev-002",
      timestamp: "2026-04-10T12:00:00Z",
      temperature: 42.0,
      humidity: 30.0,
      soil_moisture: 18.0,
    },
  },
  {
    device_id: "dev-003",
    status: "critical",
    latest: {
      device_id: "dev-003",
      timestamp: "2026-04-10T12:00:00Z",
      temperature: 65.0,
      humidity: 20.0,
      soil_moisture: 8.0,
    },
  },
];

export const mockAlerts: Alert[] = [
  {
    alert_id: "alert-001",
    device_id: "dev-003",
    rule_name: "critical_temperature",
    severity: "critical",
    triggered_at: "2026-04-10T12:00:00Z",
    resolved_at: null,
    reading: mockDevices[2].latest,
  },
  {
    alert_id: "alert-002",
    device_id: "dev-002",
    rule_name: "dry_soil",
    severity: "warning",
    triggered_at: "2026-04-10T11:55:00Z",
    resolved_at: null,
    reading: mockDevices[1].latest,
  },
  {
    alert_id: "alert-003",
    device_id: "dev-001",
    rule_name: "high_temperature",
    severity: "warning",
    triggered_at: "2026-04-10T11:30:00Z",
    resolved_at: "2026-04-10T11:45:00Z",
    reading: mockDevices[0].latest,
  },
];

export const mockHistory: TelemetryEvent[] = Array.from(
  { length: 10 },
  (_, i) => ({
    device_id: "dev-001",
    timestamp: new Date(
      Date.UTC(2026, 3, 10, 12, 0, 0) - (9 - i) * 60_000
    ).toISOString(),
    temperature: 20 + Math.sin(i * 0.5) * 5,
    humidity: 60 + Math.cos(i * 0.3) * 10,
    soil_moisture: 45 + Math.sin(i * 0.4) * 8,
  })
);

export const mockStats: DeviceStats = {
  device_count: 5,
  total_events: 12500,
  active_alerts: 2,
};
