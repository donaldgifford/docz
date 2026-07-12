import { visitParents } from "unist-util-visit-parents";

import type { ElementContent, Root, Text } from "hast";

/*
 * Cross-reference linking (IMPL-0001 Phase 4): doc-id-shaped tokens in
 * rendered bodies that resolve to a sibling document become links. The
 * resolver map is the whitelist — only ids that actually exist in the
 * repo link anywhere, and hrefs are built from API data, never from the
 * document text. Runs AFTER sanitize on trusted structure.
 */

/** UPPERCASED doc_id -> SPA href ("/owner/name/type/DOC-0001"). */
export type XrefResolver = ReadonlyMap<string, string>;

const DOC_ID_TOKEN = /\b([A-Za-z][A-Za-z0-9]*-\d+)\b/g;

/** Ancestors whose text must never be re-linked. */
const OPAQUE_TAGS = new Set(["a", "code", "pre", "script", "style"]);

export function linkifyDocIds(tree: Root, resolve: XrefResolver): void {
  if (resolve.size === 0) {
    return;
  }
  visitParents(tree, "text", (node: Text, ancestors) => {
    if (
      ancestors.some(
        (ancestor) =>
          ancestor.type === "element" && OPAQUE_TAGS.has(ancestor.tagName),
      )
    ) {
      return;
    }

    const replacement: ElementContent[] = [];
    let cursor = 0;
    for (const match of node.value.matchAll(DOC_ID_TOKEN)) {
      const token = match[0];
      const href = resolve.get(token.toUpperCase());
      if (href === undefined) {
        continue;
      }
      if (match.index > cursor) {
        replacement.push({
          type: "text",
          value: node.value.slice(cursor, match.index),
        });
      }
      replacement.push({
        type: "element",
        tagName: "a",
        properties: { href, dataXref: true },
        children: [{ type: "text", value: token }],
      });
      cursor = match.index + token.length;
    }
    if (replacement.length === 0) {
      return;
    }
    if (cursor < node.value.length) {
      replacement.push({ type: "text", value: node.value.slice(cursor) });
    }

    const parent = ancestors.at(-1);
    if (parent === undefined || !("children" in parent)) {
      return;
    }
    const index = parent.children.indexOf(node);
    if (index === -1) {
      return;
    }
    parent.children.splice(index, 1, ...(replacement as never[]));
    // Skip the nodes we just inserted.
    return index + replacement.length;
  });
}
