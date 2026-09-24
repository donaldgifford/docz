import { SKIP, visit } from "unist-util-visit";

import type { Element, Root } from "hast";

/*
 * Wraps each <table> in <div class="table-wrap"> so a table wider than
 * the reading column scrolls horizontally instead of pushing into the
 * ToC rail: a table never shrinks below its min-content width, and
 * `display: block` on the table itself would drop its semantics. Runs
 * AFTER sanitize — schema.ts strips className from document divs, so
 * the wrapper cannot be forged and the rehype-generated one is the
 * only element carrying it. Idempotent: an already-wrapped table is
 * left alone.
 */
export function rehypeWrapTables() {
  return (tree: Root): void => {
    visit(tree, "element", (node, index, parent) => {
      if (
        node.tagName !== "table" ||
        parent === undefined ||
        index === undefined
      ) {
        return;
      }
      if (
        parent.type === "element" &&
        parent.tagName === "div" &&
        hasClassName(parent.properties.className, "table-wrap")
      ) {
        return;
      }
      // Same WAI scrollable-region treatment MarkdownPre gives code
      // blocks (axe scrollable-region-focusable, serious): a named,
      // keyboard-focusable region, so the scroll is reachable by tab.
      const wrapper: Element = {
        type: "element",
        tagName: "div",
        properties: {
          className: ["table-wrap"],
          role: "region",
          ariaLabel: "table",
          tabIndex: 0,
        },
        children: [node],
      };
      parent.children[index] = wrapper;
      return [SKIP, index + 1];
    });
  };
}

function hasClassName(value: unknown, target: string): boolean {
  if (typeof value === "string") {
    return value.split(/\s+/).includes(target);
  }
  if (Array.isArray(value)) {
    return value.includes(target);
  }
  return false;
}
