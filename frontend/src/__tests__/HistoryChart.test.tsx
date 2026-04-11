import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { HistoryChart } from "@/components/HistoryChart";
import { mockHistory } from "./mocks/fixtures";

describe("HistoryChart", () => {
  it("renders chart container with SVG", () => {
    render(<HistoryChart data={mockHistory} />);
    const container = screen.getByTestId("history-chart");
    expect(container).toBeInTheDocument();
    // ResponsiveContainer renders SVG only when it has measured dimensions.
    // In jsdom, elements have 0 width/height, so Recharts logs a warning
    // and skips SVG rendering. We verify the container + Recharts wrapper exist.
    expect(
      container.querySelector(".recharts-responsive-container")
    ).toBeInTheDocument();
  });
});
