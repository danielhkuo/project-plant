import type {
  DeviceWithStatus,
  Alert,
  TelemetryEvent,
  DeviceStats,
} from "@/lib/types";

// A larger demo fleet showcasing all status states
export const demoDevices: DeviceWithStatus[] = [
  {
    device_id: "greenhouse-01",
    status: "normal",
    latest: {
      device_id: "greenhouse-01",
      timestamp: new Date().toISOString(),
      temperature: 23.4,
      humidity: 68.2,
      soil_moisture: 52.1,
    },
  },
  {
    device_id: "greenhouse-02",
    status: "normal",
    latest: {
      device_id: "greenhouse-02",
      timestamp: new Date().toISOString(),
      temperature: 25.1,
      humidity: 72.5,
      soil_moisture: 48.7,
    },
  },
  {
    device_id: "desert-bed-01",
    status: "warning",
    latest: {
      device_id: "desert-bed-01",
      timestamp: new Date().toISOString(),
      temperature: 42.3,
      humidity: 18.0,
      soil_moisture: 15.2,
    },
  },
  {
    device_id: "desert-bed-02",
    status: "warning",
    latest: {
      device_id: "desert-bed-02",
      timestamp: new Date().toISOString(),
      temperature: 41.8,
      humidity: 22.0,
      soil_moisture: 19.5,
    },
  },
  {
    device_id: "indoor-lab-01",
    status: "normal",
    latest: {
      device_id: "indoor-lab-01",
      timestamp: new Date().toISOString(),
      temperature: 21.0,
      humidity: 55.0,
      soil_moisture: 45.0,
    },
  },
  {
    device_id: "critical-plant-07",
    status: "critical",
    latest: {
      device_id: "critical-plant-07",
      timestamp: new Date().toISOString(),
      temperature: 63.2,
      humidity: 15.0,
      soil_moisture: 6.8,
    },
  },
  {
    device_id: "tropical-05",
    status: "normal",
    latest: {
      device_id: "tropical-05",
      timestamp: new Date().toISOString(),
      temperature: 28.5,
      humidity: 82.0,
      soil_moisture: 62.4,
    },
  },
  {
    device_id: "waterlogged-03",
    status: "warning",
    latest: {
      device_id: "waterlogged-03",
      timestamp: new Date().toISOString(),
      temperature: 24.0,
      humidity: 88.0,
      soil_moisture: 93.5,
    },
  },
];

export const demoAlerts: Alert[] = [
  {
    alert_id: "alert-critical-01",
    device_id: "critical-plant-07",
    rule_name: "critical_temperature",
    severity: "critical",
    triggered_at: new Date(Date.now() - 2 * 60_000).toISOString(),
    resolved_at: null,
    reading: demoDevices[5].latest,
  },
  {
    alert_id: "alert-critical-02",
    device_id: "critical-plant-07",
    rule_name: "critical_dry_soil",
    severity: "critical",
    triggered_at: new Date(Date.now() - 2 * 60_000).toISOString(),
    resolved_at: null,
    reading: demoDevices[5].latest,
  },
  {
    alert_id: "alert-warn-01",
    device_id: "desert-bed-01",
    rule_name: "high_temperature",
    severity: "warning",
    triggered_at: new Date(Date.now() - 15 * 60_000).toISOString(),
    resolved_at: null,
    reading: demoDevices[2].latest,
  },
  {
    alert_id: "alert-warn-02",
    device_id: "desert-bed-01",
    rule_name: "dry_soil",
    severity: "warning",
    triggered_at: new Date(Date.now() - 15 * 60_000).toISOString(),
    resolved_at: null,
    reading: demoDevices[2].latest,
  },
  {
    alert_id: "alert-warn-03",
    device_id: "waterlogged-03",
    rule_name: "waterlogged",
    severity: "warning",
    triggered_at: new Date(Date.now() - 30 * 60_000).toISOString(),
    resolved_at: null,
    reading: demoDevices[7].latest,
  },
  {
    alert_id: "alert-info-01",
    device_id: "desert-bed-01",
    rule_name: "low_humidity",
    severity: "info",
    triggered_at: new Date(Date.now() - 45 * 60_000).toISOString(),
    resolved_at: null,
    reading: demoDevices[2].latest,
  },
  {
    alert_id: "alert-resolved-01",
    device_id: "greenhouse-01",
    rule_name: "high_humidity",
    severity: "info",
    triggered_at: new Date(Date.now() - 2 * 60 * 60_000).toISOString(),
    resolved_at: new Date(Date.now() - 60 * 60_000).toISOString(),
    reading: demoDevices[0].latest,
  },
];

export function generateHistory(
  deviceId: string,
  hours: number = 1,
  baseTemp: number = 22,
  baseHumidity: number = 65,
  baseSoil: number = 45
): TelemetryEvent[] {
  const now = Date.now();
  const pointCount = Math.min(hours * 60, 200); // 1 per minute, max 200
  return Array.from({ length: pointCount }, (_, i) => {
    const t = now - (pointCount - 1 - i) * (hours * 60 * 60_000) / pointCount;
    return {
      device_id: deviceId,
      timestamp: new Date(t).toISOString(),
      temperature: baseTemp + Math.sin(i * 0.15) * 3 + (Math.random() - 0.5),
      humidity: baseHumidity + Math.cos(i * 0.1) * 8 + (Math.random() - 0.5) * 2,
      soil_moisture: baseSoil + Math.sin(i * 0.08) * 5 + (Math.random() - 0.5),
    };
  });
}

export const demoStats: DeviceStats = {
  device_count: demoDevices.length,
  total_events: 48_293,
  active_alerts: demoAlerts.filter((a) => a.resolved_at === null).length,
};
