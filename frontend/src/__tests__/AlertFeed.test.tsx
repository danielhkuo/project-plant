import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { AlertFeed } from "@/components/AlertFeed";
import type { Alert } from "@/lib/types";
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

  it("treats an alert without resolved_at as active", () => {
    // Payload shape from a dashboard API that omits the field for active
    // alerts instead of sending null.
    const fromApi = JSON.parse(
      JSON.stringify({ ...mockAlerts[0], resolved_at: undefined })
    ) as Alert;
    expect("resolved_at" in fromApi).toBe(false);

    render(<AlertFeed alerts={[fromApi]} onResolve={vi.fn()} />);

    expect(screen.getByRole("button", { name: /resolve/i })).toBeInTheDocument();
    expect(screen.queryByText("[RESOLVED]")).not.toBeInTheDocument();
  });
});
