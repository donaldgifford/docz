import { visit } from "unist-util-visit";

import type { Element, Root } from "hast";

/*
 * Stamps `data-line` on every block element from the source position
 * the parser recorded (docz-review's render mode, DESIGN-0024 section
 * 5). Runs AFTER sanitize: schema.ts allows no `data-line`, so an
 * author-supplied one is stripped before this plugin runs and the only
 * attribute left on the page is the pipeline's own. The positions are
 * document lines only when the source went through
 * preprocessDoczMarkdown with `preserveLines`.
 *
 * Inline elements are skipped: an anchor is built from the block a
 * selection sits in, and a stamp on every <em> would be noise.
 */
export const BLOCK_TAGS: ReadonlySet<string> = new Set([
  "address",
  "article",
  "aside",
  "blockquote",
  "details",
  "dd",
  "div",
  "dl",
  "dt",
  "figure",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "hr",
  "li",
  "ol",
  "p",
  "pre",
  "section",
  "summary",
  "table",
  "tr",
  "ul",
]);

export function rehypeDataLine() {
  return (tree: Root): void => {
    visit(tree, "element", (node) => {
      const line = node.position?.start.line;
      if (line === undefined || !BLOCK_TAGS.has(node.tagName)) {
        return;
      }
      node.properties.dataLine = String(line);
    });
  };
}

/*
 * Shiki replaces each <pre> it highlights with a fresh one and drops
 * the original's properties, the stamp included. The pair below carries
 * it across: `collect` records every <pre>'s stamp in document order
 * before Shiki, and `restore` writes them back onto the <pre>s in the
 * same order after it. Shiki emits exactly one <pre> per <pre>, so the
 * two walks line up; a count mismatch restores nothing rather than
 * stamping a wrong line.
 */
export function preLineCarrier() {
  let lines: (string | undefined)[] = [];
  const pres = (tree: Root) => {
    const found: Element[] = [];
    visit(tree, "element", (node) => {
      if (node.tagName === "pre") {
        found.push(node);
      }
    });
    return found;
  };
  return {
    collect: () => (tree: Root) => {
      lines = pres(tree).map((pre) => {
        const line = pre.properties.dataLine;
        return typeof line === "string" ? line : undefined;
      });
    },
    restore: () => (tree: Root) => {
      const after = pres(tree);
      if (after.length !== lines.length) {
        return;
      }
      after.forEach((pre, i) => {
        const line = lines[i];
        if (line !== undefined) {
          pre.properties.dataLine = line;
        }
      });
    },
  };
}
