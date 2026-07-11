import { describe, expect, it } from "vitest";

import { NEUTRAL_STATUS_COLOR, statusColor, typeColor } from "@/lib/colors";

describe("typeColor", () => {
  it.each([
    ["rfc", "var(--color-t-rfc)"],
    ["adr", "var(--color-t-adr)"],
    ["design", "var(--color-t-design)"],
    ["impl", "var(--color-t-impl)"],
    ["investigation", "var(--color-t-investigation)"],
    ["inv", "var(--color-t-investigation)"],
    ["mandate", "var(--color-t-mandate)"],
    ["guide", "var(--color-t-guide)"],
    ["principle", "var(--color-t-principle)"],
    ["policy", "var(--color-t-policy)"],
    ["framework", "var(--color-t-framework)"],
  ])("maps curated type %s", (type, expected) => {
    expect(typeColor(type)).toBe(expected);
  });

  it("is case- and whitespace-insensitive", () => {
    expect(typeColor("RFC")).toBe("var(--color-t-rfc)");
    expect(typeColor("  Design ")).toBe("var(--color-t-design)");
    expect(typeColor("INV")).toBe("var(--color-t-investigation)");
  });

  it("hashes unknown types into the fixed fallback palette", () => {
    const color = typeColor("runbook");
    expect(color).toMatch(/^var\(--color-hash-[0-7]\)$/);
  });

  it("is deterministic for unknown types across calls and casing", () => {
    expect(typeColor("runbook")).toBe(typeColor("runbook"));
    expect(typeColor("RunBook")).toBe(typeColor("runbook"));
  });

  it("pins the hash function against accidental change", () => {
    // If these move, every custom type's color changes for every user —
    // that's a breaking visual change, not a refactor.
    expect(typeColor("runbook")).toBe("var(--color-hash-7)");
    expect(typeColor("postmortem")).toBe("var(--color-hash-7)");
    expect(typeColor("playbook")).toBe("var(--color-hash-6)");
  });
});

describe("statusColor", () => {
  it.each([
    ["Draft", "var(--color-st-draft)"],
    ["open", "var(--color-st-draft)"],
    ["Proposed", "var(--color-st-proposed)"],
    ["In Review", "var(--color-st-proposed)"],
    ["in progress", "var(--color-st-proposed)"],
    ["Accepted", "var(--color-st-accepted)"],
    ["ACTIVE", "var(--color-st-accepted)"],
    ["Adopted", "var(--color-st-accepted)"],
    ["implemented", "var(--color-st-accepted)"],
    ["Concluded", "var(--color-st-accepted)"],
    ["Rejected", "var(--color-st-rejected)"],
    ["cancelled", "var(--color-st-rejected)"],
    ["canceled", "var(--color-st-rejected)"],
    ["Superseded", "var(--color-st-superseded)"],
    ["Deprecated", "var(--color-st-deprecated)"],
    ["archived", "var(--color-st-deprecated)"],
    ["Paused", "var(--color-st-deprecated)"],
  ])("maps %s by convention", (status, expected) => {
    expect(statusColor(status)).toBe(expected);
  });

  it("normalizes internal whitespace", () => {
    expect(statusColor("in    review")).toBe("var(--color-st-proposed)");
  });

  it("returns neutral for anything unrecognized", () => {
    expect(statusColor("Percolating")).toBe(NEUTRAL_STATUS_COLOR);
    expect(statusColor("")).toBe(NEUTRAL_STATUS_COLOR);
  });
});
