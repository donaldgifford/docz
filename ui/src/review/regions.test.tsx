import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";
import { ordinals } from "@/review/regions";

import type { Region } from "@/cli/__generated__/inspect";

async function reviewDom(md: string, regions: Region[]) {
  const { content } = await renderMarkdown(md, { review: true, regions });
  const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
  return container;
}

const region = (
  kind: string,
  start: number,
  end: number,
  extra: Partial<Region> = {},
): Region => ({ kind, start, end, depth: 0, closed: true, ...extra });

const DOC = [
  "<!--docz:phase:start-->", // 1
  "### Phase 1: Start", // 2
  "", // 3
  "Intro.", // 4
  "", // 5
  "<!--docz:tasks:start-->", // 6
  "- [ ] one", // 7
  "- [ ] two", // 8
  "<!--docz:tasks:end-->", // 9
  "<!--docz:phase:end-->", // 10
  "", // 11
  "<!--docz:phase:start-->", // 12
  "### Phase 2: Next", // 13
  "", // 14
  "Body.", // 15
  "<!--docz:phase:end-->", // 16
  "", // 17
  "Outside.", // 18
].join("\n");

const REGIONS = [
  region("phase", 1, 10),
  region("tasks", 6, 9, { depth: 1 }),
  region("phase", 12, 16),
];

function regionOf(container: HTMLElement, text: string) {
  const el = Array.from(container.querySelectorAll("p, li, h3")).find((n) =>
    n.textContent.includes(text),
  );
  const section = el?.closest("section[data-region]");
  return section === null || section === undefined
    ? null
    : `${section.getAttribute("data-region") ?? ""}:${
        section.getAttribute("data-ordinal") ?? ""
      }`;
}

describe("rehypeRegions", () => {
  it("wraps each region's blocks with its kind and ordinal", async () => {
    const c = await reviewDom(DOC, REGIONS);
    expect(regionOf(c, "Phase 1")).toBe("phase:0");
    expect(regionOf(c, "Intro.")).toBe("phase:0");
    expect(regionOf(c, "Body.")).toBe("phase:1");
    expect(regionOf(c, "Outside.")).toBeNull();
    const phase = c.querySelector('section[data-region="phase"]');
    expect(phase?.getAttribute("aria-label")).toBe("phase region");
  });

  it("puts a block in the deepest region that contains it", async () => {
    const c = await reviewDom(DOC, REGIONS);
    expect(regionOf(c, "one")).toBe("tasks:0");
    const tasks = c.querySelector('section[data-region="tasks"]');
    expect(tasks?.parentElement?.getAttribute("data-region")).toBe("phase");
  });

  it("leaves a region a block crosses unwrapped", async () => {
    // tasks 6..8 cuts the list (lines 7-8) in half.
    const c = await reviewDom(DOC, [
      region("phase", 1, 10),
      region("tasks", 6, 8, { depth: 1 }),
    ]);
    expect(c.querySelector('section[data-region="tasks"]')).toBeNull();
    expect(regionOf(c, "one")).toBe("phase:0");
    expect(c.textContent).toContain("two");
  });

  it("leaves a malformed region unwrapped and wraps its inner ones", async () => {
    const c = await reviewDom(DOC, [
      region("phase", 1, 10, { closed: false }),
      region("tasks", 6, 9, { depth: 1 }),
      region("phase", 16, 12),
    ]);
    expect(c.querySelector('section[data-region="phase"]')).toBeNull();
    expect(regionOf(c, "one")).toBe("tasks:0");
    expect(c.textContent).toContain("Body.");
  });

  it("renders everything unwrapped with no regions", async () => {
    const c = await reviewDom(DOC, []);
    expect(c.querySelector("section[data-region]")).toBeNull();
    expect(c.textContent).toContain("Intro.");
  });

  it("numbers ordinals per kind in document order", () => {
    const n = ordinals([
      region("phase", 12, 16),
      region("tasks", 6, 9),
      region("phase", 1, 10),
    ]);
    expect(
      [...n.entries()].map(
        ([r, i]) => `${r.kind}@${String(r.start)}=${String(i)}`,
      ),
    ).toEqual(["phase@1=0", "tasks@6=0", "phase@12=1"]);
  });
});
