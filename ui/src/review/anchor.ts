import { toString as mdastToString } from "mdast-util-to-string";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";
import { visit } from "unist-util-visit";

import { remarkGithubAlerts } from "@/markdown/github-alerts";
import { preprocessDoczMarkdown } from "@/markdown/preprocess";

import type { Anchor as RecordAnchor } from "@/cli/__generated__/review";
import type { Heading, Region } from "@/cli/__generated__/inspect";
import type { Nodes, Root } from "mdast";

/*
 * Anchors (DESIGN-0024 section 7). An annotation is anchored by a quote
 * of the page's rendered text plus its context, never by a DOM path or
 * a byte offset, so it survives the agent rewriting the document.
 *
 * Both sides read the same text. On the page, a block is the nearest
 * element carrying the review render's `data-line`, and its text is the
 * text nodes it owns (not those of a nested block). In the source, a
 * block is a paragraph, heading, code block, or table row parsed by the
 * same remark pipeline, and its text is `mdast-util-to-string` over it:
 * the text the DOM shows, with emphasis, links, code spans, entities,
 * and soft wraps normalised identically. Context lengths and offsets
 * are in Unicode code points.
 */

/** Code points of context kept on each side of a quote, within its block. */
export const CONTEXT = 32;

/** An anchor before it is resolved: the record's shape less `resolution`. */
export type Anchor = Omit<RecordAnchor, "resolution">;

export interface SelectionAnchor extends Anchor {
  /** The whole selection as the page shows it, across blocks. */
  selection: string;
  /**
   * The selection spans blocks: `exact` is cut at the first block's end,
   * and only a comment may carry it.
   */
  multiBlock: boolean;
}

export type Resolution = RecordAnchor["resolution"];

export interface Resolved {
  resolution: Resolution;
  /** The hit's line for `exact`; the anchor's own line, a hint, otherwise. */
  line: number;
  /** The region of the anchor's kind the hit is in now; else the anchor's. */
  ordinal: number;
}

const codePoints = (s: string): string[] => Array.from(s);

function blockOf(node: Node | null): HTMLElement | null {
  const el = node instanceof Element ? node : (node?.parentElement ?? null);
  return el?.closest<HTMLElement>("[data-line]") ?? null;
}

// The text nodes a block owns, in document order.
function ownText(block: HTMLElement): Text[] {
  const out: Text[] = [];
  const walker = block.ownerDocument.createTreeWalker(
    block,
    NodeFilter.SHOW_TEXT,
  );
  for (let n = walker.nextNode(); n !== null; n = walker.nextNode()) {
    if (n instanceof Text && blockOf(n) === block) {
      out.push(n);
    }
  }
  return out;
}

// The UTF-16 offset of a boundary point within a block's own text.
function offsetIn(texts: Text[], container: Node, offset: number): number {
  const point = container.ownerDocument?.createRange();
  point?.setStart(container, offset);
  let acc = 0;
  for (const t of texts) {
    if (t === container) {
      return acc + offset;
    }
    if (point === undefined || point.comparePoint(t, 0) >= 0) {
      return acc;
    }
    acc += t.data.length;
  }
  return acc;
}

/** The nearest heading at or above `line`, as `docz inspect` reports it. */
export function headingAt(headings: readonly Heading[], line: number): string {
  let found = "";
  for (const h of headings) {
    if (h.line > line) {
      break;
    }
    found = h.text;
  }
  return found;
}

/**
 * Builds the anchor for a selection on the review page. Nothing is
 * refused: a selection spanning blocks is anchored at its first block
 * and flagged, and text outside any block yields null.
 */
export function anchorFromRange(
  range: Range,
  headings: readonly Heading[],
): SelectionAnchor | null {
  const block = blockOf(range.startContainer);
  if (block === null) {
    return null;
  }
  const texts = ownText(block);
  const text = texts.map((t) => t.data).join("");
  const start = offsetIn(texts, range.startContainer, range.startOffset);
  const multiBlock = blockOf(range.endContainer) !== block;
  const end = multiBlock
    ? text.length
    : offsetIn(texts, range.endContainer, range.endOffset);

  const line = Number(block.dataset.line);
  const section = block.closest<HTMLElement>("section[data-region]");
  return {
    region: section?.dataset.region ?? "",
    ordinal: Number(section?.dataset.ordinal ?? 0),
    heading: headingAt(headings, line),
    exact: text.slice(start, end),
    prefix: codePoints(text.slice(0, start)).slice(-CONTEXT).join(""),
    suffix: codePoints(text.slice(end)).slice(0, CONTEXT).join(""),
    line,
    selection: range.toString(),
    multiBlock,
  };
}

interface SourceBlock {
  text: string;
  line: number;
  heading: string;
}

const BLOCK_TYPES = new Set(["paragraph", "heading", "code", "tableRow"]);

const parser = unified()
  .use(remarkParse)
  .use(remarkGfm)
  .use(remarkGithubAlerts);

/** The source's blocks with their rendered text, line, and heading. */
export function sourceBlocks(source: string): SourceBlock[] {
  // remark-gfm and the alerts plugin transform mdast in place.
  const tree = parser.runSync(
    parser.parse(preprocessDoczMarkdown(source, { preserveLines: true })),
  ) as Root;
  const blocks: SourceBlock[] = [];
  let heading = "";
  visit(tree, (node: Nodes) => {
    const line = node.position?.start.line;
    if (line === undefined || !BLOCK_TYPES.has(node.type)) {
      return;
    }
    const text = mdastToString(node);
    if (node.type === "heading") {
      heading = text;
    }
    blocks.push({ text, line, heading });
  });
  return blocks;
}

function commonPrefix(a: string[], b: string[]): number {
  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) {
    i += 1;
  }
  return i;
}

function commonSuffix(a: string[], b: string[]): number {
  let i = 0;
  while (
    i < a.length &&
    i < b.length &&
    a[a.length - 1 - i] === b[b.length - 1 - i]
  ) {
    i += 1;
  }
  return i;
}

interface Hit {
  line: number;
  score: number;
}

// How well an occurrence's surroundings match the anchor: the code points
// of prefix and suffix that agree, plus a matching heading, which
// outweighs any context under a different heading (carryMarkers' rule).
function score(block: SourceBlock, at: number, anchor: Anchor): number {
  const before = codePoints(block.text.slice(0, at)).slice(-CONTEXT);
  const after = codePoints(block.text.slice(at + anchor.exact.length)).slice(
    0,
    CONTEXT,
  );
  let n =
    commonSuffix(before, codePoints(anchor.prefix)) +
    commonPrefix(after, codePoints(anchor.suffix));
  if (anchor.heading !== "" && block.heading === anchor.heading) {
    n += 2 * CONTEXT;
  }
  return n;
}

function hits(blocks: readonly SourceBlock[], anchor: Anchor): Hit[] {
  const found: Hit[] = [];
  for (const block of blocks) {
    for (
      let at = block.text.indexOf(anchor.exact);
      at !== -1;
      at = block.text.indexOf(anchor.exact, at + 1)
    ) {
      found.push({ line: block.line, score: score(block, at, anchor) });
    }
  }
  return found;
}

// The one hit, or the one scoring strictly best; never the line hint.
function choose(found: readonly Hit[]): Hit | "lost" | "ambiguous" {
  const [first, ...rest] = found;
  if (first === undefined) {
    return "lost";
  }
  if (rest.length === 0) {
    return first;
  }
  let best = first;
  let tie = false;
  for (const hit of rest) {
    if (hit.score > best.score) {
      best = hit;
      tie = false;
    } else if (hit.score === best.score) {
      tie = true;
    }
  }
  return tie || best.score === 0 ? "ambiguous" : best;
}

const inside = (r: Region, line: number) => line > r.start && line < r.end;

/**
 * Resolves an anchor against the document as it is now. Hits inside the
 * regions of the anchor's kind are preferred; when there are none (the
 * region is gone, or the quote moved out of it) the whole document is
 * searched. One hit is `exact`, several are settled by context and
 * heading or are `ambiguous`, none is `lost`.
 */
export function resolveAnchor(
  anchor: Anchor,
  source: string,
  regions: readonly Region[] = [],
): Resolved {
  const unresolved = (resolution: Resolution): Resolved => ({
    resolution,
    line: anchor.line,
    ordinal: anchor.ordinal,
  });
  if (anchor.exact === "") {
    return unresolved("lost");
  }

  const all = hits(sourceBlocks(source), anchor);
  const kind = regions
    .filter((r) => anchor.region !== "" && r.kind === anchor.region)
    .sort((a, b) => a.start - b.start);
  const scoped = all.filter((h) => kind.some((r) => inside(r, h.line)));
  const picked = choose(scoped.length > 0 ? scoped : all);
  if (typeof picked === "string") {
    return unresolved(picked);
  }
  const at = kind.findIndex((r) => inside(r, picked.line));
  return {
    resolution: "exact",
    line: picked.line,
    ordinal: at === -1 ? anchor.ordinal : at,
  };
}
