import useSWR from "swr";
import { api } from "@/lib/api";
import type { DeviceWithStatus } from "@/lib/types";

export function useDevices() {
  const { data, error, isLoading, mutate } = useSWR<DeviceWithStatus[]>(
    "/api/v1/devices",
    () => api.getDevices(),
    { refreshInterval: 10_000 }
  );

  return {
    devices: data ?? [],
    error,
    isLoading,
    mutate,
  };
}
