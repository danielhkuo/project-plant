import type { DeviceStats } from "@/lib/types";

interface StatsBarProps {
  stats: DeviceStats;
}

function formatNumber(n: number): string {
  return n.toLocaleString("en-US");
}

export function StatsBar({ stats }: StatsBarProps) {
  return (
    <div className="flex items-center gap-8 border-b border-border px-4 py-3 md:gap-12">
      <div>
        <div className="label">DEVICES</div>
        <div className="font-mono text-xl text-text-primary">
          {formatNumber(stats.device_count)}
        </div>
      </div>
      <div>
        <div className="label">EVENTS</div>
        <div className="font-mono text-xl text-text-primary">
          {formatNumber(stats.total_events)}
        </div>
      </div>
      <div>
        <div className="label">ACTIVE ALERTS</div>
        <div className="font-mono text-xl text-accent">
          {formatNumber(stats.active_alerts)}
        </div>
      </div>
    </div>
  );
}
