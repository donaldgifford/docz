import { lazy, Suspense } from "react";

import type { WidgetName } from "@/markdown/widgets/fences";

// The validator, the YAML parser, and the components load only when a
// document has a widget, so the reader's eager bundle does not grow.
const WidgetView = lazy(() => import("@/markdown/widgets/widget-view"));

/*
 * A docz-flow, docz-stepper, or docz-scorecard block, routed here by
 * MarkdownPre from rehypeWidgetMarker's attributes. Until the widget
 * chunk arrives the YAML shows as code, never a blank box, as
 * MermaidBlock does for a diagram.
 */
export function WidgetBlock({
  name,
  source,
  fenceLine,
  line,
}: {
  name: WidgetName;
  source: string;
  fenceLine: number;
  line?: string;
}) {
  /* eslint-disable jsx-a11y/no-noninteractive-tabindex --
     same WAI scrollable-region treatment as MarkdownPre */
  const fallback = (
    <pre
      role="region"
      aria-label="yaml code block"
      tabIndex={0}
      data-widget-fallback="loading"
      data-line={line}
    >
      <code>{source}</code>
    </pre>
  );
  /* eslint-enable jsx-a11y/no-noninteractive-tabindex */

  return (
    <Suspense fallback={fallback}>
      <WidgetView
        name={name}
        source={source}
        fenceLine={fenceLine}
        line={line}
      />
    </Suspense>
  );
}
