import { describe, expect, it } from "vitest";

import {
  formatRelativeTime,
  formatUpdatedStamp,
  hitUpdatedAt,
} from "@/lib/updatedAt";

import type { SearchHit } from "@/api/__generated__/docz-api.schemas";

const NOW = new Date("2026-07-11T12:00:00Z");

function hit(extra: Record<string, unknown> = {}): SearchHit {
  return {
    source: "doc",
    repo: "donaldgifford/docz-site",
    doc_id: "DESIGN-0005",
    type: "design",
    title: "Reader typography",
    path: "docs/design/0005-reader-typography.md",
    status: "In Review",
    author: "donaldgifford",
    snippet: "",
    ...extra,
  };
}

describe("hitUpdatedAt", () => {
  it("returns '' for the hits today's API actually sends", () => {
    expect(hitUpdatedAt(hit())).toBe("");
  });

  it("reads the field once docz-api starts sending it", () => {
    expect(hitUpdatedAt(hit({ updated_at: "2026-09-10T13:52:00Z" }))).toBe(
      "2026-09-10T13:52:00Z",
    );
  });

  it("ignores a non-string value rather than rendering it", () => {
    // The Meilisearch record stores Unix seconds; a future wire change
    // that forwards the raw number must not reach the formatter.
    expect(hitUpdatedAt(hit({ updated_at: 1_757_512_320 }))).toBe("");
    expect(hitUpdatedAt(hit({ updated_at: null }))).toBe("");
  });
});

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
