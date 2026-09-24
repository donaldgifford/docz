import { visit } from "unist-util-visit";

import type { Blockquote, Paragraph, Root, Text } from "mdast";

/*
 * GitHub-flavored alerts → admonition divs (IMPL-0002 Phase 3).
 *
 *   > [!WARNING]
 *   > Body text
 *
 * becomes `<div class="admonition warning"><span class="adm-label">
 * Warning</span><p>Body text</p></div>` via mdast `hName` /
 * `hProperties`. All five GitHub kinds keep their own visual (OQ-2a).
 * Runs on mdast, BEFORE the sanitizer — schema.ts allows exactly
 * these classNames on div/span (value-restricted), so document HTML
 * can at most opt into the same inert styling. The marker only lifts
 * when it starts the blockquote's first text node; markers mid-text
 * or inside code stay literal, and plain blockquotes keep the
 * pull-quote treatment.
 */

// Case-insensitive like GitHub's own parser ([!note] works there).
const ALERT_RE = /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*\n?/i;

const LABELS = {
  note: "Note",
  tip: "Tip",
  important: "Important",
  warning: "Warning",
  caution: "Caution",
} as const;

type AlertKind = keyof typeof LABELS;

interface HastData {
  hName?: string;
  hProperties?: Record<string, unknown>;
}

export function remarkGithubAlerts() {
  return (tree: Root): void => {
    visit(tree, "blockquote", (node: Blockquote) => {
      const first: Paragraph | undefined =
        node.children[0]?.type === "paragraph" ? node.children[0] : undefined;
      if (first === undefined) {
        return;
      }
      const firstText: Text | undefined =
        first.children[0]?.type === "text" ? first.children[0] : undefined;
      if (firstText === undefined) {
        return;
      }

      const match = ALERT_RE.exec(firstText.value);
      const kind = match?.[1]?.toLowerCase() as AlertKind | undefined;
      if (match === null || kind === undefined) {
        return;
      }

      // Strip the marker; drop the leading paragraph if it held only
      // the marker (the `> [!NOTE]` line of a multi-paragraph alert).
      const remainder = firstText.value.slice(match[0].length);
      if (remainder.length > 0) {
        firstText.value = remainder;
      } else if (first.children.length > 1) {
        first.children = first.children.slice(1);
      } else {
        node.children.shift();
      }

      const labelParagraph: Paragraph = {
        type: "paragraph",
        children: [{ type: "text", value: LABELS[kind] }],
        data: {
          hName: "span",
          hProperties: { className: ["adm-label"] },
        } satisfies HastData,
      };
      node.children.unshift(labelParagraph);

      const data = (node.data ??= {}) as HastData;
      data.hName = "div";
      data.hProperties = { className: ["admonition", kind] };
    });
  };
}
