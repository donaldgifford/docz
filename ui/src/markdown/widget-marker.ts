import { toString as hastToString } from "hast-util-to-string";
import { visit } from "unist-util-visit";

import { WIDGET_FENCES } from "@/markdown/widgets/fences";

import type { Element, Root } from "hast";

/*
 * Tags ```docz-flow, ```docz-stepper, and ```docz-scorecard fences for
 * the widget components (IMPL-0030 Phase 5), as rehypeMermaidMarker
 * tags mermaid. Runs POST-sanitize and PRE-Shiki, for the same two
 * reasons:
 *
 *   - post-sanitize means document HTML can never supply the marker:
 *     sanitize strips every data-* attribute, so only a real fence
 *     reaches <WidgetBlock>, and sanitize never sees component markup;
 *   - stripping the `language-docz-*` class keeps Shiki from replacing
 *     the <pre> and losing the marker.
 *
 * The block's YAML moves onto the <pre> as `data-widget-source`, its
 * widget as `data-widget`, and the fence's line as `data-widget-line`
 * (the line the parser recorded, which is the document's when the
 * source went through preprocessDoczMarkdown with `preserveLines`, as
 * both the reader and the review page do), so an invalid block's errors
 * name the same lines `docz export html` names. The source stays the
 * <pre>'s child, so anything that does not route through MarkdownPre
 * still shows the YAML.
 */
export function rehypeWidgetMarker() {
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
      if (!Array.isArray(classes)) {
        return;
      }
      const fence = classes.find(
        (name): name is string =>
          typeof name === "string" &&
          name.startsWith("language-") &&
          Object.hasOwn(WIDGET_FENCES, name.slice("language-".length)),
      );
      if (fence === undefined) {
        return;
      }

      node.properties.dataWidget =
        WIDGET_FENCES[fence.slice("language-".length)];
      node.properties.dataWidgetSource = hastToString(code);
      const line = node.position?.start.line;
      if (line !== undefined) {
        node.properties.dataWidgetLine = String(line);
      }
      code.properties.className = classes.filter((name) => name !== fence);
    });
  };
}
