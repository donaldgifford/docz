import { toString as hastToString } from "hast-util-to-string";
import { visit } from "unist-util-visit";

import type { Element, Root } from "hast";

/*
 * Tags ```mermaid fences for client-side diagram rendering
 * (IMPL-0002 Phase 4). Runs POST-sanitize and PRE-Shiki:
 *
 *   - post-sanitize means zero schema surface — document HTML can't
 *     smuggle `data-mermaid-source` in (sanitize strips data-*), so
 *     only a real fence reaches `<MermaidBlock>`;
 *   - pre-Shiki matters because stripping `language-mermaid` off the
 *     inner <code> keeps Shiki (and with it the codeblock chrome)
 *     from replacing the <pre> and losing the marker.
 *
 * The diagram source moves onto the <pre> as `data-mermaid-source`
 * (hast-to-JSX hands it to MarkdownPre as a plain prop); the fence
 * meta — already charset-validated by schema.ts — rides along as
 * `data-mermaid-caption`. The source text stays as the <pre>'s child,
 * so anything that doesn't route through MarkdownPre still shows the
 * code.
 */
export function rehypeMermaidMarker() {
  return (tree: Root): void => {
    visit(tree, "element", (node) => {
      if (node.tagName !== "pre") {
        return;
      }
      const code = node.children.find(
        (child): child is Element => child.type === "element",
      );
      if (code?.tagName !== "code") {
        return;
      }
      const classes = code.properties.className;
      if (!Array.isArray(classes) || !classes.includes("language-mermaid")) {
        return;
      }

      const source = hastToString(code).trim();
      if (source.length === 0) {
        return;
      }
      node.properties.dataMermaidSource = source;

      const caption = code.properties.metastring;
      if (typeof caption === "string" && caption.length > 0) {
        node.properties.dataMermaidCaption = caption;
      }

      code.properties.className = classes.filter(
        (name) => name !== "language-mermaid",
      );
    });
  };
}
