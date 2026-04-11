import type { DeviceStatus } from "@/lib/types";
import { STATUS_BG_CLASS } from "@/lib/status";

interface StatusIndicatorProps {
  status: DeviceStatus;
}

export function StatusIndicator({ status }: StatusIndicatorProps) {
  return (
    <span
      data-testid="status-indicator"
      className={`inline-block h-2 w-2 rounded-full ${STATUS_BG_CLASS[status]}`}
    />
  );
}
