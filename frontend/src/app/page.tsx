"use client";

import { useCallback } from "react";
import { DeviceGrid } from "@/components/DeviceGrid";
import { StatsBar } from "@/components/StatsBar";
import { useDevices } from "@/hooks/useDevices";
import { useStats } from "@/hooks/useStats";
import { useWebSocket } from "@/hooks/useWebSocket";
import type { TelemetryEvent, Alert } from "@/lib/types";

export default function DashboardPage() {
  const { devices, isLoading, mutate } = useDevices();
  const { stats } = useStats();

  const onReading = useCallback(
    (event: TelemetryEvent) => {
      mutate(
        (current) =>
          current?.map((d) =>
            d.device_id === event.device_id ? { ...d, latest: event } : d
          ),
        { revalidate: false }
      );
    },
    [mutate]
  );

  const onAlert = useCallback((_alert: Alert) => {
    // Alerts page handles its own data; just log for now
  }, []);

  const { connected } = useWebSocket({ onReading, onAlert });

  if (isLoading) {
    return (
      <div className="py-24 text-center font-mono text-sm text-text-disabled">
        [LOADING...]
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="font-display text-4xl text-text-display">FLEET</h1>
        <span
          className={`font-mono text-[11px] uppercase tracking-widest ${
            connected ? "text-success" : "text-text-disabled"
          }`}
        >
          {connected ? "LIVE" : "CONNECTING..."}
        </span>
      </div>
      {stats && <StatsBar stats={stats} />}
      <DeviceGrid devices={devices} />
    </div>
  );
}
