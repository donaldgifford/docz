import { describe, expect, it } from "vitest";

import { formatRelativeTime, formatUpdatedStamp } from "@/lib/updatedAt";

const NOW = new Date("2026-07-11T12:00:00Z");

describe("formatUpdatedStamp", () => {
  it("splits an RFC3339 stamp into date and time lines", () => {
    expect(formatUpdatedStamp("2026-09-10T13:52:00Z", "UTC")).toEqual({
      date: "Sep 10, 2026",
      time: "1:52 PM",
    });
  });

  it("renders in the requested zone", () => {
    expect(
      formatUpdatedStamp("2026-09-10T13:52:00Z", "America/Denver"),
    ).toEqual({ date: "Sep 10, 2026", time: "7:52 AM" });
  });

  it("pads the minute and keeps a 12-hour clock", () => {
    expect(formatUpdatedStamp("2026-01-02T00:05:00Z", "UTC")).toEqual({
      date: "Jan 2, 2026",
      time: "12:05 AM",
    });
  });

  it.each(["", "not-a-date"])("returns undefined for %o", (iso) => {
    expect(formatUpdatedStamp(iso, "UTC")).toBeUndefined();
  });

  it("degrades rather than rendering 'Invalid Date' for a missing field", () => {
    // `updated_at` is typed required, so TypeScript says this cannot
    // happen — but a deployment pointed at a pre-1.5.0 docz-api simply
    // omits the property, and undefined arrives where a string was
    // promised. The NaN guard is what makes that render the em dash.
    const missing = undefined as unknown as string;
    expect(formatUpdatedStamp(missing, "UTC")).toBeUndefined();
  });
});

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
