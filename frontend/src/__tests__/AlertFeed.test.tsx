import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { AlertFeed } from "@/components/AlertFeed";
import { mockAlerts } from "./mocks/fixtures";

describe("AlertFeed", () => {
  it("renders alert list sorted by time", () => {
    render(<AlertFeed alerts={mockAlerts} onResolve={vi.fn()} />);
    const rows = screen.getAllByTestId("alert-row");
    expect(rows).toHaveLength(3);
    // Most recent first
    expect(rows[0]).toHaveTextContent("critical_temperature");
    expect(rows[1]).toHaveTextContent("dry_soil");
    expect(rows[2]).toHaveTextContent("high_temperature");
    // Severity and device info visible
    expect(screen.getByText("dev-003")).toBeInTheDocument();
    expect(screen.getByText("CRITICAL")).toBeInTheDocument();
    expect(screen.getAllByText("WARNING")).toHaveLength(2);
  });

  it("calls onResolve with correct alert_id when resolve is clicked", async () => {
    const user = userEvent.setup();
    const onResolve = vi.fn();
    render(<AlertFeed alerts={mockAlerts} onResolve={onResolve} />);
    // Only active (unresolved) alerts have resolve buttons
    const resolveButtons = screen.getAllByRole("button", { name: /resolve/i });
    expect(resolveButtons).toHaveLength(2); // alert-001 and alert-002 are active
    await user.click(resolveButtons[0]);
    expect(onResolve).toHaveBeenCalledWith("alert-001");
  });
});
