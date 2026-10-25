import { render } from "@testing-library/react";
import { createElement } from "react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { renderMarkdown } from "@/markdown/processor";
import { anchorFromRange, resolveAnchor, type Anchor } from "@/review/anchor";

import type { Heading, Region } from "@/cli/__generated__/inspect";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(() => Promise.reject(new Error("jsdom"))),
  },
}));

const anchor = (exact: string, extra: Partial<Anchor> = {}): Anchor => ({
  region: "",
  ordinal: 0,
  heading: "",
  exact,
  prefix: "",
  suffix: "",
  line: 1,
  ...extra,
});

// Ported from pkg/export/confluence's carryMarkers corpus: the same
// rules decide where a quote goes on both surfaces.
describe("resolveAnchor (carryMarkers rules)", () => {
  const doc = [
    "## Alpha", // 1
    "", // 2
    "The cache is warm. The cache is cold.", // 3
    "", // 4
    "## Beta", // 5
    "", // 6
    "The cache is warm.", // 7
  ].join("\n");

  it("resolves the one occurrence", () => {
    expect(resolveAnchor(anchor("cold"), doc)).toEqual({
      resolution: "exact",
      line: 3,
      ordinal: 0,
    });
  });

  it("picks among several by context", () => {
    const got = resolveAnchor(
      anchor("cache", { prefix: "warm. The ", suffix: " is cold" }),
      doc,
    );
    expect(got).toMatchObject({ resolution: "exact", line: 3 });
  });

  it("picks among several by heading", () => {
    const got = resolveAnchor(
      anchor("The cache is warm.", { heading: "Beta", line: 3 }),
      doc,
    );
    expect(got).toMatchObject({ resolution: "exact", line: 7 });
  });

  it("calls a tie ambiguous, whatever the line hint says", () => {
    const got = resolveAnchor(anchor("The cache is warm.", { line: 7 }), doc);
    expect(got).toEqual({ resolution: "ambiguous", line: 7, ordinal: 0 });
  });

  it("reports a missing quote lost with the hint", () => {
    const got = resolveAnchor(anchor("tepid", { line: 3 }), doc);
    expect(got).toEqual({ resolution: "lost", line: 3, ordinal: 0 });
  });
});

describe("resolveAnchor over rendered text", () => {
  it.each([
    ["emphasis", "We use **Postgres** here.", "use Postgres"],
    ["a link", "See [the design](./0001-x.md) first.", "See the design"],
    ["a soft wrap", "one line\nwraps onto the next", "line\nwraps"],
    ["an entity", "Tom &amp; Jerry", "Tom & Jerry"],
    ["a code span", "Run `docz inspect` now.", "Run docz inspect now"],
    ["combining marks", "Café au lait", "Café au"],
    ["an emoji", "Ship it 🚀 today", "it 🚀 to"],
  ])("finds a quote across %s", (_name, source, exact) => {
    expect(resolveAnchor(anchor(exact), source)).toMatchObject({
      resolution: "exact",
      line: 1,
    });
  });

  it("finds a moved region by heading, reporting where it is now", () => {
    const before: Region[] = [
      { kind: "phase", start: 1, end: 5, depth: 0, closed: true },
    ];
    const moved = [
      "<!--docz:phase:start-->", // 1
      "### Phase 1: New", // 2
      "", // 3
      "Shared sentence.", // 4
      "<!--docz:phase:end-->", // 5
      "<!--docz:phase:start-->", // 6
      "### Phase 2: Old", // 7
      "", // 8
      "Shared sentence.", // 9
      "<!--docz:phase:end-->", // 10
    ].join("\n");
    const regions: Region[] = [
      ...before,
      { kind: "phase", start: 6, end: 10, depth: 0, closed: true },
    ];
    const got = resolveAnchor(
      anchor("Shared sentence.", {
        region: "phase",
        ordinal: 0,
        heading: "Phase 2: Old",
        line: 4,
      }),
      moved,
      regions,
    );
    expect(got).toEqual({ resolution: "exact", line: 9, ordinal: 1 });
  });
});

// Selects `needle` in the container's text, across elements and blocks,
// or from the start of `needle` to the end of `through` when given.
function select(
  container: HTMLElement,
  needle: string,
  through?: string,
): Range {
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT);
  const nodes: Text[] = [];
  for (let n = walker.nextNode(); n !== null; n = walker.nextNode()) {
    if (n instanceof Text) {
      nodes.push(n);
    }
  }
  const all = nodes.map((n) => n.data).join("");
  const from = all.indexOf(needle);
  if (from === -1) {
    throw new Error(`"${needle}" is not on the page`);
  }
  const to =
    through === undefined
      ? from + needle.length
      : all.indexOf(through, from) + through.length;
  const range = document.createRange();
  let acc = 0;
  for (const n of nodes) {
    const end = acc + n.data.length;
    if (from >= acc && from <= end) {
      range.setStart(n, from - acc);
    }
    if (to >= acc && to <= end) {
      range.setEnd(n, to - acc);
      break;
    }
    acc = end;
  }
  return range;
}

const PAGE = [
  "---", // 1
  "id: DESIGN-0001", // 2
  "---", // 3
  "", // 4
  "## Goals", // 5
  "", // 6
  "<!--docz:goals:start-->", // 7
  "We use **Postgres** for the store,", // 8
  "and Valkey 🚀 for the queue.", // 9
  "<!--docz:goals:end-->", // 10
  "", // 11
  "Second paragraph here.", // 12
].join("\n");

const HEADINGS: Heading[] = [
  { level: 2, text: "Goals", anchor: "goals", line: 5 },
];
const REGIONS: Region[] = [
  { kind: "goals", start: 7, end: 10, depth: 0, closed: true },
];

async function page() {
  const { content } = await renderMarkdown(PAGE, {
    review: true,
    regions: REGIONS,
  });
  return render(createElement(MemoryRouter, null, content)).container;
}

describe("anchorFromRange", () => {
  it("anchors a formatted, wrapped, emoji selection that resolves", async () => {
    const c = await page();
    const got = anchorFromRange(
      select(c, "Postgres for the store,\nand Valkey 🚀"),
      HEADINGS,
    );
    expect(got).toEqual({
      region: "goals",
      ordinal: 0,
      heading: "Goals",
      exact: "Postgres for the store,\nand Valkey 🚀",
      prefix: "We use ",
      suffix: " for the queue.",
      line: 8,
      selection: "Postgres for the store,\nand Valkey 🚀",
      multiBlock: false,
    });
    if (got === null) {
      return;
    }
    expect(resolveAnchor(got, PAGE, REGIONS)).toEqual({
      resolution: "exact",
      line: 8,
      ordinal: 0,
    });
  });

  it("keeps context to 32 code points", async () => {
    const c = await page();
    const got = anchorFromRange(select(c, "queue"), HEADINGS);
    expect(got?.prefix).toBe("the store,\nand Valkey 🚀 for the ");
    expect(Array.from(got?.prefix ?? "")).toHaveLength(32);
  });

  it("anchors a multi-block selection at its first block", async () => {
    const c = await page();
    const got = anchorFromRange(select(c, "queue.", "Second"), HEADINGS);
    expect(got).toMatchObject({
      exact: "queue.",
      line: 8,
      multiBlock: true,
      region: "goals",
    });
    expect(got?.selection).toContain("Second");
  });

  it("anchors text outside any region with an empty region", async () => {
    const c = await page();
    const got = anchorFromRange(select(c, "Second"), HEADINGS);
    expect(got).toMatchObject({ region: "", ordinal: 0, line: 12 });
  });
});
