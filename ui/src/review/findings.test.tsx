import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { Findings, toggleFinding } from "@/review/findings";
import { StatusSelect } from "@/review/status";
import { FINDINGS, INSPECT } from "@/review/testing";

import type { FindingDraft } from "@/review/draft";

function Harness({ onActions }: { onActions: (next: FindingDraft[]) => void }) {
  const [actions, setActions] = useState<FindingDraft[]>([]);
  return (
    <Findings
      findings={FINDINGS}
      actions={actions}
      onChange={(next) => {
        onActions(next);
        setActions(next);
      }}
    />
  );
}

function row(code: string): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-finding="${code}"]`);
  if (el === null) {
    throw new Error(`no finding ${code}`);
  }
  return el;
}

describe("Findings", () => {
  it("shows each finding with its severity, code, line, and detail", () => {
    render(<Harness onActions={vi.fn()} />);
    expect(row("region.missing")).toHaveAttribute("data-severity", "error");
    expect(row("region.missing")).not.toHaveTextContent("line 0");
    expect(row("design.goals.empty")).toHaveTextContent(
      "warningdesign.goals.emptyline 12the goals section has no goals in it",
    );
  });

  it("dismisses, fixes, switches, and clears", async () => {
    const user = userEvent.setup();
    const seen = vi.fn<(next: FindingDraft[]) => void>();
    render(<Harness onActions={seen} />);
    const goals = within(row("design.goals.empty"));
    await user.click(goals.getByRole("button", { name: "Dismiss" }));
    expect(seen.mock.lastCall?.[0]).toEqual([
      { code: "design.goals.empty", line: 12, action: "dismiss" },
    ]);
    await user.click(goals.getByRole("button", { name: "Fix this" }));
    expect(goals.getByRole("button", { name: "Fix this" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(seen.mock.lastCall?.[0]).toEqual([
      { code: "design.goals.empty", line: 12, action: "fix" },
    ]);
    await user.click(goals.getByRole("button", { name: "Fix this" }));
    expect(seen.mock.lastCall?.[0]).toEqual([]);
  });

  it("says so when there is nothing", () => {
    render(<Findings findings={[]} actions={[]} onChange={vi.fn()} />);
    expect(screen.getByText(/reported nothing/)).toBeInTheDocument();
  });

  it("keys a finding by code and line", () => {
    const a = toggleFinding([], { code: "x", line: 1 }, "fix");
    const b = toggleFinding(a, { code: "x", line: 2 }, "dismiss");
    expect(b).toHaveLength(2);
  });
});

describe("StatusSelect", () => {
  it("offers the type's statuses with the current one marked", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <StatusSelect
        statuses={INSPECT.statuses}
        current="Draft"
        value="Draft"
        onChange={onChange}
      />,
    );
    const control = within(screen.getByRole("tablist", { name: "Status" }));
    expect(
      control.getByRole("tab", { name: "Draft (current)" }),
    ).toHaveAttribute("aria-selected", "true");
    await user.click(control.getByRole("tab", { name: "In Review" }));
    expect(onChange).toHaveBeenCalledWith("In Review");
  });

  it("shows the chosen status, which may not be the current one", () => {
    render(
      <StatusSelect
        statuses={INSPECT.statuses}
        current="Draft"
        value="In Review"
        onChange={vi.fn()}
      />,
    );
    expect(screen.getByRole("tab", { name: "In Review" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("tab", { name: "Draft (current)" }),
    ).toHaveAttribute("aria-selected", "false");
  });

  it("keeps a current status the type does not list", () => {
    render(
      <StatusSelect
        statuses={["Draft"]}
        current="Odd"
        value="Odd"
        onChange={vi.fn()}
      />,
    );
    expect(screen.getAllByRole("tab").map((o) => o.textContent)).toEqual([
      "Odd (current)",
      "Draft",
    ]);
  });

  it("adds no segment for a document with no status", () => {
    render(
      <StatusSelect
        statuses={["Draft", "Accepted"]}
        current=""
        value=""
        onChange={vi.fn()}
      />,
    );
    expect(screen.getAllByRole("tab").map((o) => o.textContent)).toEqual([
      "Draft",
      "Accepted",
    ]);
  });
});
