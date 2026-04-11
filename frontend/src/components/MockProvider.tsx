"use client";

import { useEffect, useState } from "react";
import { useSWRConfig } from "swr";

const DEMO_MODE = process.env.NEXT_PUBLIC_DEMO_MODE === "true";

export function MockProvider({ children }: { children: React.ReactNode }) {
  // In demo mode, block render until the service worker is ready so that
  // hooks don't fire real fetches against /api/v1/... before MSW intercepts.
  const [ready, setReady] = useState(!DEMO_MODE);
  const { mutate } = useSWRConfig();

  useEffect(() => {
    if (!DEMO_MODE) return;

    let cancelled = false;
    let tickInterval: ReturnType<typeof setInterval> | null = null;

    (async () => {
      const { worker } = await import("@/mocks/browser");
      const { tickDemoReadings } = await import("@/mocks/handlers");

      await worker.start({
        onUnhandledRequest: "bypass",
        quiet: true,
      });

      if (cancelled) return;
      setReady(true);

      // Simulate live telemetry: every 2s nudge readings and invalidate
      // the SWR caches so the dashboard feels alive without a backend.
      tickInterval = setInterval(() => {
        tickDemoReadings();
        mutate("/api/v1/devices");
        mutate("/api/v1/stats");
      }, 2000);
    })();

    return () => {
      cancelled = true;
      if (tickInterval) clearInterval(tickInterval);
    };
  }, [mutate]);

  if (!ready) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <span className="label text-text-secondary">[STARTING DEMO...]</span>
      </div>
    );
  }

  return <>{children}</>;
}
