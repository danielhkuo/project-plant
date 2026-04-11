import useSWR from "swr";
import { api } from "@/lib/api";
import type { TelemetryEvent } from "@/lib/types";

interface HistoryParams {
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export function useDeviceHistory(
  deviceId: string | null,
  params?: HistoryParams
) {
  const key = deviceId
    ? `/api/v1/devices/${deviceId}/history?${JSON.stringify(params)}`
    : null;

  const { data, error, isLoading } = useSWR<TelemetryEvent[]>(key, () =>
    api.getDeviceHistory(deviceId!, params)
  );

  return {
    history: data ?? [],
    error,
    isLoading,
  };
}
