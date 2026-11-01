import { useMemo } from "react";

import { Flow } from "@/markdown/widgets/flow";
import { Scorecard } from "@/markdown/widgets/scorecard";
import { Stepper } from "@/markdown/widgets/stepper";
import { validateWidget, type WidgetName } from "@/markdown/widgets/validate";

/*
 * The lazy half of a widget block (IMPL-0030 Phase 5): validation and
 * the three components, with the YAML parser, in one chunk that loads
 * only on a document that has a widget. A valid block draws as a figure;
 * an invalid one is a caution callout naming each error at its document
 * line, then the YAML, as `docz export html` writes it. Every string
 * reaches the page as a React text child, never as markup.
 */
export interface WidgetViewProps {
  name: WidgetName;
  source: string;
  /** The fence's document line; the YAML starts on the next one. */
  fenceLine: number;
  /** The review render's `data-line` stamp, carried to the root. */
  line?: string;
}

const FENCE: Readonly<Record<WidgetName, string>> = {
  flow: "docz-flow",
  stepper: "docz-stepper",
  scorecard: "docz-scorecard",
};

export default function WidgetView({
  name,
  source,
  fenceLine,
  line,
}: WidgetViewProps) {
  const result = useMemo(
    () => validateWidget(name, source, fenceLine + 1),
    [name, source, fenceLine],
  );

  if (!result.ok) {
    return (
      <div className="widget-invalid" data-line={line}>
        <div className="admonition caution">
          <span className="adm-label">Invalid {FENCE[name]} block</span>
          <ul>
            {result.errors.map((e, i) => (
              <li key={`${String(i)}-${e}`}>{e}</li>
            ))}
          </ul>
        </div>
        {/* eslint-disable jsx-a11y/no-noninteractive-tabindex --
            the WAI scrollable-region treatment MarkdownPre gives code */}
        <pre role="region" aria-label="yaml code block" tabIndex={0}>
          <code>{source}</code>
        </pre>
        {/* eslint-enable jsx-a11y/no-noninteractive-tabindex */}
      </div>
    );
  }

  const { widget } = result;
  const title = widget.data.title;
  return (
    <figure
      className="widget my-6 border border-border-default bg-bg-base p-4"
      data-widget={widget.name}
      data-line={line}
    >
      {title !== undefined && (
        <figcaption className="mb-3 font-semibold text-fg-primary">
          {title}
        </figcaption>
      )}
      {widget.name === "flow" && <Flow data={widget.data} />}
      {widget.name === "stepper" && <Stepper data={widget.data} />}
      {widget.name === "scorecard" && <Scorecard data={widget.data} />}
    </figure>
  );
}
