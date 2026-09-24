import { visit } from "unist-util-visit";

import type { Root } from "mdast";

/*
 * Lifts the fenced-code meta string (```go internal/ingest/parse.go)
 * onto the generated hast <code> as a real `metastring` property.
 * mdast-util-to-hast only carries meta in node.data, which neither the
 * rehype-raw round trip nor rehype-sanitize preserves — a property
 * survives both. schema.ts allows `metastring` on code behind a strict
 * value pattern (conservative charset, length cap), so a hostile meta
 * loses the caption while the code block itself always renders.
 * @shikijs/rehype reads `properties.metastring` and hands it to
 * transformers as `this.options.meta.__raw`, where the codeblock
 * chrome transformer in processor.ts turns it into `data-caption`.
 */
export function remarkCaptureCodeMeta() {
  return (tree: Root): void => {
    visit(tree, "code", (node) => {
      const meta = node.meta?.trim();
      if (meta === undefined || meta.length === 0) {
        return;
      }
      node.data ??= {};
      // hProperties is mdast-util-to-hast's documented pass-through;
      // the mdast types don't declare it.
      const data = node.data as { hProperties?: Record<string, unknown> };
      data.hProperties ??= {};
      data.hProperties.metastring = meta;
    });
  };
}
