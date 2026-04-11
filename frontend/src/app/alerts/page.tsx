"use client";

import { useState } from "react";
import { AlertFeed } from "@/components/AlertFeed";
import { useAlerts } from "@/hooks/useAlerts";
import { api } from "@/lib/api";

type SeverityFilter = "all" | "info" | "warning" | "critical";
type StatusFilter = "all" | "active" | "resolved";

const SEVERITY_OPTIONS: SeverityFilter[] = ["all", "info", "warning", "critical"];
const STATUS_OPTIONS: StatusFilter[] = ["all", "active", "resolved"];

export default function AlertsPage() {
  const [severity, setSeverity] = useState<SeverityFilter>("all");
  const [status, setStatus] = useState<StatusFilter>("active");

  const { alerts, isLoading, mutate } = useAlerts({
    severity: severity === "all" ? undefined : severity,
    status: status === "all" ? undefined : status,
  });

  const onResolve = async (alertId: string) => {
    await api.resolveAlert(alertId);
    mutate();
  };

  return (
    <div className="space-y-6">
      <h1 className="font-display text-4xl text-text-display">ALERTS</h1>

      {/* Filters */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:gap-8">
        <div>
          <div className="label mb-2">SEVERITY</div>
          <div className="inline-flex rounded-full border border-border-visible">
            {SEVERITY_OPTIONS.map((opt) => (
              <button
                key={opt}
                onClick={() => setSeverity(opt)}
                className={`rounded-full px-4 py-2 font-mono text-[11px] uppercase tracking-widest transition-colors ${
                  severity === opt
                    ? "bg-text-display text-black"
                    : "text-text-secondary hover:text-text-primary"
                }`}
              >
                {opt}
              </button>
            ))}
          </div>
        </div>
        <div>
          <div className="label mb-2">STATUS</div>
          <div className="inline-flex rounded-full border border-border-visible">
            {STATUS_OPTIONS.map((opt) => (
              <button
                key={opt}
                onClick={() => setStatus(opt)}
                className={`rounded-full px-4 py-2 font-mono text-[11px] uppercase tracking-widest transition-colors ${
                  status === opt
                    ? "bg-text-display text-black"
                    : "text-text-secondary hover:text-text-primary"
                }`}
              >
                {opt}
              </button>
            ))}
          </div>
        </div>
      </div>

      {isLoading ? (
        <div className="py-24 text-center font-mono text-sm text-text-disabled">
          [LOADING...]
        </div>
      ) : (
        <AlertFeed alerts={alerts} onResolve={onResolve} />
      )}
    </div>
  );
}
