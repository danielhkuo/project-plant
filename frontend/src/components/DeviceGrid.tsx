import type { DeviceWithStatus } from "@/lib/types";
import { DeviceCard } from "./DeviceCard";

interface DeviceGridProps {
  devices: DeviceWithStatus[];
}

export function DeviceGrid({ devices }: DeviceGridProps) {
  if (devices.length === 0) {
    return (
      <div className="py-24 text-center font-mono text-sm text-text-disabled">
        [NO DEVICES]
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {devices.map((device) => (
        <DeviceCard key={device.device_id} device={device} />
      ))}
    </div>
  );
}
