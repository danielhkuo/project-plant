import useSWR from "swr";
import { api } from "@/lib/api";
import type { DeviceStats } from "@/lib/types";

export function useStats() {
  const { data, error, isLoading } = useSWR<DeviceStats>(
    "/api/v1/stats",
    () => api.getStats(),
    { refreshInterval: 10_000 }
  );

  return {
    stats: data ?? null,
    error,
    isLoading,
  };
}
