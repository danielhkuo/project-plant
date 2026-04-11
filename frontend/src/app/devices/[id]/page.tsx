"use client";

import { use, useState } from "react";
import Link from "next/link";
import useSWR from "swr";
import { api } from "@/lib/api";
import { HistoryChart } from "@/components/HistoryChart";
import { AlertFeed } from "@/components/AlertFeed";
import { StatusIndicator } from "@/components/StatusIndicator";
import { useDeviceHistory } from "@/hooks/useDeviceHistory";
import { useAlerts } from "@/hooks/useAlerts";
import { STATUS_TEXT_CLASS } from "@/lib/status";
import type { DeviceWithStatus } from "@/lib/types";

const RANGES = [
  { label: "1H", hours: 1 },
  { label: "6H", hours: 6 },
  { label: "24H", hours: 24 },
  { label: "7D", hours: 168 },
];

export default function DeviceDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const [rangeHours, setRangeHours] = useState(1);

  const { data: device } = useSWR<DeviceWithStatus>(
    `/api/v1/devices/${id}`,
    () => api.getDevice(id)
  );

  const historyParams = {
    from: new Date(Date.now() - rangeHours * 60 * 60 * 1000).toISOString(),
    to: new Date().toISOString(),
    limit: 500,
  };
  const { history } = useDeviceHistory(id, historyParams);
  const { alerts, mutate: mutateAlerts } = useAlerts({ device_id: id });

  const onResolve = async (alertId: string) => {
    await api.resolveAlert(alertId);
    mutateAlerts();
  };

  if (!device) {
    return (
      <div className="py-24 text-center font-mono text-sm text-text-disabled">
        [LOADING...]
      </div>
    );
  }

  const valueColor = STATUS_TEXT_CLASS[device.status];

  return (
    <div className="space-y-8">
      {/* Back button */}
      <Link
        href="/"
        className="inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-widest text-text-disabled transition-colors hover:text-text-primary"
      >
        &lt; BACK
      </Link>

      {/* Header */}
      <div className="flex items-center gap-4">
        <StatusIndicator status={device.status} />
        <h1 className="font-display text-4xl text-text-display">
          {device.device_id}
        </h1>
      </div>

      {/* Current readings — hero numbers */}
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <div>
          <div className="label mb-2">TEMPERATURE</div>
          <div className={`font-mono text-5xl tabular-nums ${valueColor}`}>
            {device.latest.temperature.toFixed(1)}
          </div>
          <div className="label mt-1">°C</div>
        </div>
        <div>
          <div className="label mb-2">HUMIDITY</div>
          <div className={`font-mono text-5xl tabular-nums ${valueColor}`}>
            {device.latest.humidity.toFixed(1)}
          </div>
          <div className="label mt-1">%</div>
        </div>
        <div>
          <div className="label mb-2">SOIL MOISTURE</div>
          <div className={`font-mono text-5xl tabular-nums ${valueColor}`}>
            {device.latest.soil_moisture.toFixed(1)}
          </div>
          <div className="label mt-1">%</div>
        </div>
      </div>

      {/* Time range selector */}
      <div>
        <div className="label mb-3">HISTORY</div>
        <div className="mb-4 inline-flex rounded-full border border-border-visible">
          {RANGES.map(({ label, hours }) => (
            <button
              key={label}
              onClick={() => setRangeHours(hours)}
              className={`rounded-full px-4 py-2 font-mono text-[11px] uppercase tracking-widest transition-colors ${
                rangeHours === hours
                  ? "bg-text-display text-black"
                  : "text-text-secondary hover:text-text-primary"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
        <HistoryChart data={history} />
      </div>

      {/* Device alerts */}
      <div>
        <div className="label mb-3">DEVICE ALERTS</div>
        <AlertFeed alerts={alerts} onResolve={onResolve} />
      </div>
    </div>
  );
}
