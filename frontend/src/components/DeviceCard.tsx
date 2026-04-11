import Link from "next/link";
import type { DeviceWithStatus } from "@/lib/types";
import { STATUS_TEXT_CLASS } from "@/lib/status";
import { StatusIndicator } from "./StatusIndicator";

interface DeviceCardProps {
  device: DeviceWithStatus;
}

function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  return d.toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: "UTC",
  });
}

function formatValue(n: number): string {
  return n.toFixed(1);
}

export function DeviceCard({ device }: DeviceCardProps) {
  const { device_id, status, latest } = device;
  const valueColor = STATUS_TEXT_CLASS[status];

  return (
    <Link href={`/devices/${device_id}`} data-testid="device-card">
      <div className="rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-visible">
        <div className="mb-4 flex items-center justify-between">
          <span className="font-body text-base text-text-primary">
            {device_id}
          </span>
          <StatusIndicator status={status} />
        </div>

        <div className="grid grid-cols-3 gap-3">
          <div>
            <div className="label mb-1">TEMP</div>
            <div className={`truncate font-mono text-lg tabular-nums ${valueColor}`}>
              {formatValue(latest.temperature)}
            </div>
            <div className="label">°C</div>
          </div>
          <div>
            <div className="label mb-1">HUMIDITY</div>
            <div className={`truncate font-mono text-lg tabular-nums ${valueColor}`}>
              {formatValue(latest.humidity)}
            </div>
            <div className="label">%</div>
          </div>
          <div>
            <div className="label mb-1">SOIL</div>
            <div className={`truncate font-mono text-lg tabular-nums ${valueColor}`}>
              {formatValue(latest.soil_moisture)}
            </div>
            <div className="label">%</div>
          </div>
        </div>

        <div className="mt-4 font-mono text-xs text-text-secondary">
          {formatTimestamp(latest.timestamp)}
        </div>
      </div>
    </Link>
  );
}
