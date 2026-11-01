import { useState } from "react";

import type { FlowData } from "@/markdown/widgets/validate";

/*
 * docz-flow: the stages in order, each a button that shows its detail.
 * The export's runtime draws the same data (pkg/theme/runtime.js
 * drawFlow); here every colour is a token utility, so a theme or mode
 * change restyles it with no redraw.
 */
export function Flow({ data }: { data: FlowData }) {
  const [hot, setHot] = useState<number | null>(null);
  const gates = new Set(data.gates ?? []);
  const stage = hot === null ? undefined : data.stages[hot];

  return (
    <div className="flex flex-col gap-3">
      {data.label !== undefined && (
        <p className="text-[13px] text-fg-tertiary">{data.label}</p>
      )}
      <ol
        className="flex flex-wrap items-stretch gap-2"
        aria-label={data.title ?? "Flow"}
      >
        {data.stages.map((st, i) => {
          const gate = gates.has(i + 1);
          const on = hot === i;
          return (
            <li
              key={`${st.name}-${String(i)}`}
              className="flex items-center gap-2"
            >
              <button
                type="button"
                aria-pressed={on}
                onClick={() => {
                  setHot(i);
                }}
                className={[
                  "min-w-24 border px-3 py-2 text-left text-[13px] font-semibold",
                  on
                    ? "border-accent bg-accent text-(--color-on-accent)"
                    : gate
                      ? "border-2 border-(--color-ok) bg-bg-raised text-fg-primary"
                      : "border-border-default bg-bg-raised text-fg-primary",
                ].join(" ")}
              >
                {st.name}
                {gate && <span className="sr-only"> (gate)</span>}
              </button>
              {i < data.stages.length - 1 && (
                <span aria-hidden="true" className="text-fg-secondary">
                  →
                </span>
              )}
            </li>
          );
        })}
      </ol>
      <div aria-live="polite" className="text-[14px]">
        {stage === undefined ? (
          <p className="text-fg-tertiary">Select a stage.</p>
        ) : (
          <>
            <p className="font-semibold text-fg-primary">{stage.name}</p>
            <p className="text-fg-prose">{stage.does}</p>
            {stage.note !== undefined && (
              <p className="text-[13px] text-fg-tertiary">{stage.note}</p>
            )}
          </>
        )}
      </div>
      {data.legend !== undefined && (
        <p className="text-[13px] text-fg-secondary">{data.legend}</p>
      )}
      {data.alert !== undefined && (
        <p className="text-[13px] text-(--color-bad)">{data.alert}</p>
      )}
    </div>
  );
}
