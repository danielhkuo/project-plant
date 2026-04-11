"use client";

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import type { TelemetryEvent } from "@/lib/types";

interface HistoryChartProps {
  data: TelemetryEvent[];
}

function formatTime(ts: string): string {
  const d = new Date(ts);
  return d.toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone: "UTC",
  });
}

export function HistoryChart({ data }: HistoryChartProps) {
  const chartData = data.map((e) => ({
    time: formatTime(e.timestamp),
    temperature: Number(e.temperature.toFixed(1)),
    humidity: Number(e.humidity.toFixed(1)),
    soil_moisture: Number(e.soil_moisture.toFixed(1)),
  }));

  return (
    <div data-testid="history-chart" className="h-80 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={chartData}
          margin={{ top: 8, right: 8, left: 0, bottom: 0 }}
        >
          <CartesianGrid
            horizontal={true}
            vertical={false}
            stroke="var(--color-border)"
          />
          <XAxis
            dataKey="time"
            tick={{ fontSize: 11, fontFamily: "var(--font-mono)", fill: "var(--color-text-secondary)" }}
            stroke="var(--color-border)"
          />
          <YAxis
            tick={{ fontSize: 11, fontFamily: "var(--font-mono)", fill: "var(--color-text-secondary)" }}
            stroke="var(--color-border)"
          />
          <Tooltip
            contentStyle={{
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border-visible)",
              borderRadius: 8,
              fontFamily: "var(--font-mono)",
              fontSize: 12,
            }}
          />
          <Line
            type="monotone"
            dataKey="temperature"
            stroke="var(--color-accent)"
            strokeWidth={2}
            dot={false}
            name="Temp (°C)"
          />
          <Line
            type="monotone"
            dataKey="humidity"
            stroke="var(--color-interactive)"
            strokeWidth={2}
            dot={false}
            name="Humidity (%)"
          />
          <Line
            type="monotone"
            dataKey="soil_moisture"
            stroke="var(--color-success)"
            strokeWidth={2}
            dot={false}
            name="Soil (%)"
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
