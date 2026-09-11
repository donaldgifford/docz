import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBadge, StatusPill } from "@/components/badges";

describe("StatusBadge", () => {
  it("renders label plus color dot", () => {
    render(<StatusBadge status="Accepted" />);
    expect(screen.getByText("Accepted")).toBeInTheDocument();
    expect(screen.getByTestId("status-dot")).toHaveAttribute("aria-hidden");
    expect(screen.getByText("Accepted").style.getPropertyValue("--c")).toBe(
      "var(--color-st-accepted)",
    );
  });

  it("uses the neutral color for unknown statuses", () => {
    render(<StatusBadge status="Percolating" />);
    expect(screen.getByText("Percolating").style.getPropertyValue("--c")).toBe(
      "var(--color-fg-muted)",
    );
  });
});

describe("StatusPill", () => {
  it("renders the status with its convention color", () => {
    render(<StatusPill status="Draft" />);
    expect(screen.getByText("Draft").style.getPropertyValue("--c")).toBe(
      "var(--color-st-draft)",
    );
  });
});
