import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { DeviceGrid } from "@/components/DeviceGrid";
import { DeviceCard } from "@/components/DeviceCard";
import { mockDevices } from "./mocks/fixtures";

describe("DeviceGrid", () => {
  it("renders N device cards from mock data", () => {
    render(<DeviceGrid devices={mockDevices} />);
    const cards = screen.getAllByTestId("device-card");
    expect(cards).toHaveLength(3);
    expect(screen.getByText("dev-001")).toBeInTheDocument();
    expect(screen.getByText("dev-002")).toBeInTheDocument();
    expect(screen.getByText("dev-003")).toBeInTheDocument();
  });
});

describe("DeviceCard", () => {
  it("shows device_id, latest temp/humidity/moisture, and timestamp", () => {
    render(<DeviceCard device={mockDevices[0]} />);
    expect(screen.getByText("dev-001")).toBeInTheDocument();
    expect(screen.getByText("22.5")).toBeInTheDocument();
    expect(screen.getByText("65.0")).toBeInTheDocument();
    expect(screen.getByText("45.0")).toBeInTheDocument();
    expect(screen.getByText(/2026/)).toBeInTheDocument();
  });

  it("applies correct status color coding", () => {
    const { unmount } = render(<DeviceCard device={mockDevices[0]} />);
    expect(screen.getByTestId("status-indicator")).toHaveClass("bg-success");
    unmount();

    const { unmount: unmount2 } = render(
      <DeviceCard device={mockDevices[1]} />
    );
    expect(screen.getByTestId("status-indicator")).toHaveClass("bg-warning");
    unmount2();

    render(<DeviceCard device={mockDevices[2]} />);
    expect(screen.getByTestId("status-indicator")).toHaveClass("bg-accent");
  });
});
