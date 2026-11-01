import { MermaidBlock } from "@/markdown/mermaid-block";
import { WidgetBlock } from "@/markdown/widget-block";
import { WIDGET_FENCES, type WidgetName } from "@/markdown/widgets/fences";

import type { ComponentPropsWithoutRef } from "react";

/*
 * Code blocks scroll horizontally when they overflow, and whether they
 * overflow depends on the reader's font metrics — so every block gets
 * the WAI scrollable-region treatment up front (axe
 * scrollable-region-focusable, serious): keyboard-focusable, named
 * region. Mapped over `pre` in the pipeline's JSX step, after
 * sanitize, so document HTML can never supply these attributes itself
 * — data-language only ever comes from the Shiki chrome transformer
 * (schema.ts strips it from document HTML), so the region name can
 * trust it.
 */
export function MarkdownPre({
  "data-language": lang,
  "data-mermaid-source": mermaidSource,
  "data-mermaid-caption": mermaidCaption,
  "data-widget": widget,
  "data-widget-source": widgetSource,
  "data-widget-line": widgetLine,
  ...rest
}: ComponentPropsWithoutRef<"pre"> & {
  "data-language"?: string;
  "data-mermaid-source"?: string;
  "data-mermaid-caption"?: string;
  "data-widget"?: string;
  "data-widget-source"?: string;
  "data-widget-line"?: string;
  "data-line"?: string;
}) {
  // Marked by rehypeWidgetMarker, post-sanitize like the mermaid marker,
  // so only a real docz-* fence carries these.
  if (typeof widget === "string" && typeof widgetSource === "string") {
    const name = Object.values(WIDGET_FENCES).find(
      (candidate): candidate is WidgetName => candidate === widget,
    );
    if (name !== undefined) {
      return (
        <WidgetBlock
          name={name}
          source={widgetSource}
          fenceLine={Number(widgetLine ?? "1") || 1}
          line={rest["data-line"]}
        />
      );
    }
  }
  // Marked by rehypeMermaidMarker (post-sanitize — only real fences
  // carry this); the block owns its own semantics and fallback.
  if (typeof mermaidSource === "string" && mermaidSource.length > 0) {
    return (
      <MermaidBlock
        source={mermaidSource}
        caption={mermaidCaption}
        line={rest["data-line"]}
      />
    );
  }
  /* eslint-disable jsx-a11y/no-noninteractive-tabindex --
     tabIndex on a named region is the WAI pattern for scrollable code
     blocks; without it keyboard users can't scroll them. */
  return (
    <pre
      role="region"
      aria-label={lang === undefined ? "code block" : `${lang} code block`}
      tabIndex={0}
      data-language={lang}
      {...rest}
    />
  );
  /* eslint-enable jsx-a11y/no-noninteractive-tabindex */
}
