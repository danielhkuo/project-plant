import useSWR from "swr";
import { api } from "@/lib/api";
import type { Alert } from "@/lib/types";

interface AlertFilters {
  severity?: string;
  status?: string;
  device_id?: string;
}

export function useAlerts(filters?: AlertFilters) {
  const key = filters
    ? `/api/v1/alerts?${JSON.stringify(filters)}`
    : "/api/v1/alerts";

  const { data, error, isLoading, mutate } = useSWR<Alert[]>(key, () =>
    api.getAlerts(filters)
  );

  return {
    alerts: data ?? [],
    error,
    isLoading,
    mutate,
  };
}
