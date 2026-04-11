import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { StatsBar } from "@/components/StatsBar";
import { mockStats } from "./mocks/fixtures";

describe("StatsBar", () => {
  it("shows device count, total events, and active alerts with labels", () => {
    render(<StatsBar stats={mockStats} />);
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("12,500")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("DEVICES")).toBeInTheDocument();
    expect(screen.getByText("EVENTS")).toBeInTheDocument();
    expect(screen.getByText("ACTIVE ALERTS")).toBeInTheDocument();
  });
});
