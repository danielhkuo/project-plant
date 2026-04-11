import type { DeviceStatus, AlertSeverity } from "./types";

export const STATUS_COLOR: Record<DeviceStatus, string> = {
  normal: "var(--color-success)",
  warning: "var(--color-warning)",
  critical: "var(--color-accent)",
  stale: "var(--color-text-disabled)",
};

export const STATUS_TEXT_CLASS: Record<DeviceStatus, string> = {
  normal: "text-success",
  warning: "text-warning",
  critical: "text-accent",
  stale: "text-text-disabled",
};

export const STATUS_BG_CLASS: Record<DeviceStatus, string> = {
  normal: "bg-success",
  warning: "bg-warning",
  critical: "bg-accent",
  stale: "bg-text-disabled",
};

export const SEVERITY_TEXT_CLASS: Record<AlertSeverity, string> = {
  info: "text-text-secondary",
  warning: "text-warning",
  critical: "text-accent",
};

export const SEVERITY_BORDER_CLASS: Record<AlertSeverity, string> = {
  info: "border-text-secondary",
  warning: "border-warning",
  critical: "border-accent",
};
