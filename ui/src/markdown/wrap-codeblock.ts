import { SKIP, visit } from "unist-util-visit";

import type { Element, ElementContent, Root } from "hast";

/*
 * Wraps each highlighted `<pre data-language="…">` in the codeblock
 * chrome from the mockup treatment (INV-0001 Observation 4):
 *
 *   <div class="codeblock">
 *     <div class="codeblock-header">
 *       <span class="lang">go</span>
 *       <span class="caption">internal/ingest/parse.go</span>  ← optional
 *     </div>
 *     <pre data-language="go" …>…</pre>
 *   </div>
 *
 * Runs AFTER sanitize on structure this pipeline generated: only the
 * Shiki transformer in processor.ts stamps `data-language` /
 * `data-caption`, and schema.ts strips those from document HTML, so
 * the chrome cannot be forged. Skips mermaid-marked blocks (they own
 * their container from Phase 4 on) and plain/unknown-language fences,
 * which stay bare `<pre>`. Idempotent: an already-wrapped pre is left
 * alone.
 */
export function rehypeWrapCodeblocks() {
  return (tree: Root): void => {
    visit(tree, "element", (node, index, parent) => {
      if (
        node.tagName !== "pre" ||
        parent === undefined ||
        index === undefined
      ) {
        return;
      }
      const lang = readStringProperty(node.properties.dataLanguage);
      if (lang === null || lang === "mermaid") {
        return;
      }
      if (node.properties.dataMermaidSource !== undefined) {
        return;
      }
      if (
        parent.type === "element" &&
        parent.tagName === "div" &&
        hasClassName(parent.properties.className, "codeblock")
      ) {
        return;
      }

      const caption = readStringProperty(node.properties.dataCaption);
      const headerChildren: ElementContent[] = [
        {
          type: "element",
          tagName: "span",
          properties: { className: ["lang"] },
          children: [{ type: "text", value: lang }],
        },
      ];
      if (caption !== null) {
        headerChildren.push({
          type: "element",
          tagName: "span",
          properties: { className: ["caption"] },
          children: [{ type: "text", value: caption }],
        });
      }

      const wrapper: Element = {
        type: "element",
        tagName: "div",
        properties: { className: ["codeblock"] },
        children: [
          {
            type: "element",
            tagName: "div",
            properties: { className: ["codeblock-header"] },
            children: headerChildren,
          },
          node,
        ],
      };
      parent.children[index] = wrapper;
      return [SKIP, index + 1];
    });
  };
}

function readStringProperty(value: unknown): string | null {
  return typeof value === "string" && value.length > 0 ? value : null;
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
