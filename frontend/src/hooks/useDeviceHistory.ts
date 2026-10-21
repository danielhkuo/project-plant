import useSWR from "swr";
import { api } from "@/lib/api";
import type { TelemetryEvent } from "@/lib/types";

const HISTORY_LIMIT = 500;
const REFRESH_INTERVAL_MS = 30_000;

type HistoryKey = readonly ["history", string, number];

// The fetcher takes its inputs from the SWR key, not from the component's
// closure, so a response is always stored under the key it was fetched for.
async function fetchHistory([, deviceId, rangeHours]: HistoryKey) {
  const to = Date.now();
  const events = await api.getDeviceHistory(deviceId, {
    from: new Date(to - rangeHours * 60 * 60 * 1000).toISOString(),
    to: new Date(to).toISOString(),
    limit: HISTORY_LIMIT,
  });
  // The chart's time axis runs left to right. The API sends newest-first and
  // the demo mock oldest-first, so sort instead of assuming either. Compare
  // parsed times: RFC 3339 strings with fractional seconds don't sort as text.
  return events
    .slice()
    .sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp));
}

// The SWR key must stay the same between renders. Keying on a from/to window
// computed from Date.now() made every render a new key, so each response
// re-rendered the page and started another fetch. Key on what the user chose
// (device and range) and compute the window inside the fetcher instead; the
// refresh interval then moves the window forward every 30 seconds.
export function useDeviceHistory(deviceId: string | null, rangeHours: number) {
  const { data, error, isLoading } = useSWR<
    TelemetryEvent[],
    Error,
    HistoryKey | null
  >(deviceId ? ["history", deviceId, rangeHours] : null, fetchHistory, {
    refreshInterval: REFRESH_INTERVAL_MS,
  });

  return {
    history: data ?? [],
    error,
    isLoading,
  };
}
