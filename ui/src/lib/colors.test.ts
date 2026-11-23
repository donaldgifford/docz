import { describe, expect, it } from "vitest";

import { NEUTRAL_STATUS_VARIANT, statusVariant, typeColor } from "@/lib/colors";

describe("typeColor", () => {
  it.each([
    ["rfc", "var(--color-t-rfc)"],
    ["adr", "var(--color-t-adr)"],
    ["design", "var(--color-t-design)"],
    ["impl", "var(--color-t-impl)"],
    ["investigation", "var(--color-t-investigation)"],
    ["inv", "var(--color-t-investigation)"],
    ["runbook", "var(--color-t-runbook)"],
    ["rb", "var(--color-t-runbook)"],
    ["mandate", "var(--color-t-mandate)"],
    ["guide", "var(--color-t-guide)"],
    ["principle", "var(--color-t-principle)"],
    ["policy", "var(--color-t-policy)"],
    ["framework", "var(--color-t-framework)"],
    ["docs", "var(--color-t-docs)"],
  ])("maps curated type %s", (type, expected) => {
    expect(typeColor(type)).toBe(expected);
  });

  it("curates runbook rather than hashing it (DESIGN-0019)", () => {
    expect(typeColor("RUNBOOK")).toBe("var(--color-t-runbook)");
    expect(typeColor("runbook")).not.toMatch(/--color-hash-/);
  });

  it("is case- and whitespace-insensitive", () => {
    expect(typeColor("RFC")).toBe("var(--color-t-rfc)");
    expect(typeColor("  Design ")).toBe("var(--color-t-design)");
    expect(typeColor("INV")).toBe("var(--color-t-investigation)");
  });

  it("hashes unknown types into the fixed fallback palette", () => {
    const color = typeColor("postmortem");
    expect(color).toMatch(
      /^var\(--color-hash-[0-7], var\(--category-[1-8]\)\)$/,
    );
  });

  it("is deterministic for unknown types across calls and casing", () => {
    expect(typeColor("postmortem")).toBe(typeColor("postmortem"));
    expect(typeColor("PostMortem")).toBe(typeColor("postmortem"));
  });

  it("pins the hash function against accidental change", () => {
    // If these move, every custom type's color changes for every user —
    // that's a breaking visual change, not a refactor.
    expect(typeColor("postmortem")).toBe(
      "var(--color-hash-7, var(--category-8))",
    );
    expect(typeColor("playbook")).toBe(
      "var(--color-hash-6, var(--category-7))",
    );
  });
});

describe("statusVariant", () => {
  // DESIGN-0028 section 8's table, every status in every group.
  it.each([
    ["Draft", "warning"],
    ["Open", "warning"],
    ["Proposed", "info"],
    ["In Review", "info"],
    ["In Progress", "info"],
    ["Accepted", "success"],
    ["Active", "success"],
    ["Approved", "success"],
    ["Adopted", "success"],
    ["Completed", "success"],
    ["Implemented", "success"],
    ["Concluded", "success"],
    ["Rejected", "destructive"],
    ["Cancelled", "destructive"],
    ["Canceled", "destructive"],
    ["Abandoned", "destructive"],
    ["Superseded", "ignored"],
    ["Deprecated", "outline"],
    ["Archived", "outline"],
    ["Paused", "outline"],
    ["Inconclusive", "outline"],
  ] as const)("maps %s to %s", (status, expected) => {
    expect(statusVariant(status)).toBe(expected);
  });

  it("ignores case and extra whitespace", () => {
    expect(statusVariant("ACCEPTED")).toBe("success");
    expect(statusVariant("  in    review ")).toBe("info");
  });

  it("is neutral for anything outside the convention", () => {
    expect(NEUTRAL_STATUS_VARIANT).toBe("outline");
    expect(statusVariant("Percolating")).toBe(NEUTRAL_STATUS_VARIANT);
    expect(statusVariant("")).toBe(NEUTRAL_STATUS_VARIANT);
  });

  it("gives Superseded the package's ignored colour, not a status role", () => {
    // The one visible change of the move (design-system#32): it was a
    // violet of docz's own.
    expect(statusVariant("Superseded")).toBe("ignored");
  });
});
