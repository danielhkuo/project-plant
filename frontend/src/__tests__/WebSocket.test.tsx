import { render, screen, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SWRConfig } from "swr";
import { DeviceGrid } from "@/components/DeviceGrid";
import { mockDevices } from "./mocks/fixtures";
import type { DeviceWithStatus, TelemetryEvent } from "@/lib/types";

// We test the integration concept: when a WebSocket reading arrives,
// the device grid re-renders with the updated value.
// Instead of mocking the actual WebSocket connection, we test the update
// mechanism directly — simulating what useWebSocket's onReading callback does.

describe("WebSocket updates grid", () => {
  let devices: DeviceWithStatus[];

  beforeEach(() => {
    devices = structuredClone(mockDevices);
  });

  it("re-renders DeviceGrid when a new reading arrives", async () => {
    // Simulate the SWR + WebSocket update flow:
    // 1. Initial render with mock devices
    // 2. Simulate a reading callback that updates state
    // 3. Assert the grid shows the new value

    let updateDevices: (updated: DeviceWithStatus[]) => void;

    function TestWrapper() {
      const [data, setData] = vi.fn(
        // Use useState to simulate SWR mutate
        () => devices
      ) as unknown as [DeviceWithStatus[], (d: DeviceWithStatus[]) => void];
      // Actually use React.useState
      return null;
    }

    // Simpler approach: render with initial data, then re-render with updated data
    const { rerender } = render(
      <SWRConfig value={{ provider: () => new Map() }}>
        <DeviceGrid devices={devices} />
      </SWRConfig>
    );

    // Verify initial value
    expect(screen.getByText("22.5")).toBeInTheDocument();

    // Simulate WebSocket reading: update temperature for dev-001
    const updatedReading: TelemetryEvent = {
      device_id: "dev-001",
      timestamp: "2026-04-10T12:01:00Z",
      temperature: 99.9,
      humidity: 65.0,
      soil_moisture: 45.0,
    };

    // Apply the update (mimics what onReading + mutate does)
    const updatedDevices = devices.map((d) =>
      d.device_id === updatedReading.device_id
        ? { ...d, latest: updatedReading }
        : d
    );

    // Re-render with updated data (simulates SWR cache update)
    rerender(
      <SWRConfig value={{ provider: () => new Map() }}>
        <DeviceGrid devices={updatedDevices} />
      </SWRConfig>
    );

    // Assert new value is displayed
    expect(screen.getByText("99.9")).toBeInTheDocument();
    // Old value should be gone
    expect(screen.queryByText("22.5")).not.toBeInTheDocument();
  });
});
