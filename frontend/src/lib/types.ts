export interface TelemetryEvent {
  device_id: string;
  timestamp: string;
  temperature: number;
  humidity: number;
  soil_moisture: number;
}

export type DeviceStatus = "normal" | "warning" | "critical" | "stale";

export interface DeviceWithStatus {
  device_id: string;
  status: DeviceStatus;
  latest: TelemetryEvent;
}

export type AlertSeverity = "info" | "warning" | "critical";

export interface Alert {
  alert_id: string;
  device_id: string;
  rule_name: string;
  severity: AlertSeverity;
  triggered_at: string;
  resolved_at: string | null;
  reading: TelemetryEvent;
}

export interface DeviceStats {
  device_count: number;
  total_events: number;
  active_alerts: number;
}

export interface WebSocketMessage {
  type: "welcome" | "reading" | "alert";
  device_id?: string;
  payload: TelemetryEvent | Alert | { message: string };
}
