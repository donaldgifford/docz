import type { Region } from "@/cli/__generated__/inspect";
import type { Element, ElementContent, Root, RootContent } from "hast";

/*
 * Wraps the review render's blocks in one labelled
 * `<section data-region="<kind>" data-ordinal="<n>">` per region that
 * `docz inspect` reported (DESIGN-0024 section 5). A region's content is
 * the lines strictly between its two marker lines (or, for an inferred
 * region, the synthetic lines docz reports the same way), and a block
 * belongs to a region when its whole source span lies inside them.
 *
 * Regions nest, so the wrap walks depth-first: the outermost regions of
 * a scope are wrapped over its children, then each one's inner regions
 * over that section's children, so a block ends up in the deepest
 * region that contains it. Nothing is ever hidden: a malformed region
 * (unclosed, or ending before it starts) and a region a block crosses
 * are left unwrapped, and their inner regions are tried in the same
 * scope instead.
 *
 * Runs post-sanitize on the pipeline's own tree, so the attributes
 * cannot be forged, and before Shiki, while every block still carries
 * the `position` the parser gave it.
 */

/** The `n` in `data-ordinal`: which region of its kind, in document order. */
export function ordinals(regions: readonly Region[]): Map<Region, number> {
  const sorted = [...regions].sort((a, b) => a.start - b.start);
  const seen = new Map<string, number>();
  const out = new Map<Region, number>();
  for (const region of sorted) {
    const n = seen.get(region.kind) ?? 0;
    out.set(region, n);
    seen.set(region.kind, n + 1);
  }
  return out;
}

function contains(outer: Region, inner: Region): boolean {
  return (
    outer !== inner &&
    outer.start <= inner.start &&
    inner.end <= outer.end &&
    !(outer.start === inner.start && outer.end === inner.end)
  );
}

function outermost(regions: readonly Region[]): Region[] {
  return regions
    .filter((r) => !regions.some((o) => contains(o, r)))
    .sort((a, b) => a.start - b.start);
}

function span(node: RootContent): [number, number] | undefined {
  const pos = node.position;
  return pos === undefined ? undefined : [pos.start.line, pos.end.line];
}

// The run of children a region wraps, as [first, last] indices, or
// undefined when the region wraps nothing or a block crosses it.
function members(
  children: readonly RootContent[],
  region: Region,
): [number, number] | undefined {
  let first = -1;
  let last = -1;
  for (const [i, child] of children.entries()) {
    const s = span(child);
    if (s === undefined) {
      continue;
    }
    const [from, to] = s;
    const inside = from > region.start && to < region.end;
    const overlaps = from < region.end && to > region.start;
    if (inside) {
      if (first === -1) {
        first = i;
      }
      last = i;
    } else if (overlaps) {
      return undefined;
    }
  }
  return first === -1 ? undefined : [first, last];
}

function wrapScope(
  children: RootContent[],
  regions: readonly Region[],
  ordinal: Map<Region, number>,
): RootContent[] {
  let out = children;
  for (const region of outermost(regions)) {
    const inner = regions.filter((r) => contains(region, r));
    const wellFormed = region.closed && region.end > region.start;
    const run = wellFormed ? members(out, region) : undefined;
    if (run === undefined) {
      out = wrapScope(out, inner, ordinal);
      continue;
    }
    const [first, last] = run;
    const section: Element = {
      type: "element",
      tagName: "section",
      properties: {
        dataRegion: region.kind,
        dataOrdinal: String(ordinal.get(region) ?? 0),
        ariaLabel: `${region.kind} region`,
      },
      children: wrapScope(
        out.slice(first, last + 1),
        inner,
        ordinal,
      ) as ElementContent[],
    };
    out = [...out.slice(0, first), section, ...out.slice(last + 1)];
  }
  return out;
}

export function rehypeRegions(regions: readonly Region[]) {
  return (tree: Root): void => {
    if (regions.length === 0) {
      return;
    }
    tree.children = wrapScope(tree.children, regions, ordinals(regions));
  };
}
