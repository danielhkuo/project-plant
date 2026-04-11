import type { Alert } from "@/lib/types";
import { SEVERITY_TEXT_CLASS, SEVERITY_BORDER_CLASS } from "@/lib/status";

interface AlertFeedProps {
  alerts: Alert[];
  onResolve: (alertId: string) => void;
}

function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  return d.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: "UTC",
  });
}

export function AlertFeed({ alerts, onResolve }: AlertFeedProps) {
  const sorted = [...alerts].sort(
    (a, b) =>
      new Date(b.triggered_at).getTime() - new Date(a.triggered_at).getTime()
  );

  if (sorted.length === 0) {
    return (
      <div className="py-24 text-center font-mono text-sm text-text-disabled">
        [NO ALERTS]
      </div>
    );
  }

  return (
    <div>
      {sorted.map((alert) => {
        const isResolved = alert.resolved_at !== null;
        return (
          <div
            key={alert.alert_id}
            data-testid="alert-row"
            className="flex items-center justify-between border-b border-border px-2 py-3"
          >
            <div className="flex items-center gap-3">
              <span
                className={`rounded-full border px-3 py-0.5 font-mono text-[11px] uppercase tracking-widest ${
                  SEVERITY_BORDER_CLASS[alert.severity]
                } ${SEVERITY_TEXT_CLASS[alert.severity]}`}
              >
                {alert.severity.toUpperCase()}
              </span>
              <span className="font-mono text-sm text-text-primary">
                {alert.device_id}
              </span>
              <span className="font-mono text-sm text-text-secondary">
                {alert.rule_name}
              </span>
            </div>

            <div className="flex items-center gap-4">
              <span className="font-mono text-xs text-text-disabled">
                {formatTimestamp(alert.triggered_at)}
              </span>
              {isResolved ? (
                <span className="font-mono text-[11px] uppercase tracking-widest text-text-disabled">
                  [RESOLVED]
                </span>
              ) : (
                <button
                  onClick={() => onResolve(alert.alert_id)}
                  className="rounded-full border border-border-visible px-4 py-1 font-mono text-[13px] uppercase tracking-[0.06em] text-text-primary transition-colors hover:border-text-primary"
                >
                  RESOLVE
                </button>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
