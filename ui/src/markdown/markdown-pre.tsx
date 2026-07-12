import type { ComponentPropsWithoutRef } from "react";

/*
 * Code blocks scroll horizontally when they overflow, and whether they
 * overflow depends on the reader's font metrics — so every block gets
 * the WAI scrollable-region treatment up front (axe
 * scrollable-region-focusable, serious): keyboard-focusable, named
 * region. Mapped over `pre` in the pipeline's JSX step, after
 * sanitize, so document HTML can never supply these attributes itself.
 */
export function MarkdownPre(props: ComponentPropsWithoutRef<"pre">) {
  /* eslint-disable jsx-a11y/no-noninteractive-tabindex --
     tabIndex on a named region is the WAI pattern for scrollable code
     blocks; without it keyboard users can't scroll them. */
  return <pre role="region" aria-label="code block" tabIndex={0} {...props} />;
  /* eslint-enable jsx-a11y/no-noninteractive-tabindex */
}
