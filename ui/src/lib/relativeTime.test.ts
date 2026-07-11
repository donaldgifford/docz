import { describe, expect, it } from "vitest";

import { formatRelativeTime } from "@/lib/relativeTime";

const NOW = new Date("2026-07-11T12:00:00Z");

describe("formatRelativeTime", () => {
  it.each([
    ["", "—"],
    ["not-a-date", "—"],
    ["2026-07-11T11:59:30Z", "just now"],
    // Future timestamp (clock skew) also reads as "just now".
    ["2026-07-11T12:00:05Z", "just now"],
    ["2026-07-11T11:15:00Z", "45m ago"],
    ["2026-07-11T04:00:00Z", "8h ago"],
    ["2026-07-09T12:00:00Z", "2d ago"],
    ["2026-06-20T12:00:00Z", "3w ago"],
    ["2026-03-11T12:00:00Z", "4mo ago"],
    ["2024-07-11T12:00:00Z", "2y ago"],
  ])("formats %s as %s", (iso, expected) => {
    expect(formatRelativeTime(iso, NOW)).toBe(expected);
  });
});
