import { useState } from "react";

import type { StepperData } from "@/markdown/widgets/validate";

/*
 * docz-stepper: pick a scenario, then walk its steps. A scenario that
 * fails at a step marks it, and every step after it is never reached.
 * The same data the export's runtime draws (drawStepper), in tokens.
 */
export function Stepper({ data }: { data: StepperData }) {
  const [scen, setScen] = useState(0);
  const [step, setStep] = useState(0);
  const scenario = data.scenarios[scen] ?? data.scenarios[0];
  const fail =
    scenario?.fails_at === undefined
      ? -1
      : data.steps.findIndex((st) => st.key === scenario.fails_at);
  const current = data.steps[step] ?? data.steps[0];
  if (scenario === undefined || current === undefined) {
    return null;
  }
  const note = scenario.notes?.[current.key];
  const failing = step === fail;

  return (
    <div className="flex flex-col gap-3 text-[14px]">
      <div role="group" aria-label="Scenario" className="flex flex-wrap gap-2">
        {data.scenarios.map((sc, i) => (
          <button
            key={sc.name}
            type="button"
            aria-pressed={i === scen}
            onClick={() => {
              setScen(i);
              setStep(0);
            }}
            className={pill(i === scen)}
          >
            {sc.name}
          </button>
        ))}
      </div>
      <ol aria-label="Step" className="flex flex-wrap gap-2">
        {data.steps.map((st, i) => {
          const dead = fail > -1 && i > fail;
          return (
            <li key={st.key}>
              <button
                type="button"
                aria-pressed={i === step}
                onClick={() => {
                  setStep(i);
                }}
                className={[
                  pill(i === step),
                  i === fail ? "border-(--color-bad)" : "",
                  dead ? "opacity-70" : "",
                ].join(" ")}
              >
                <span className="font-mono text-[12px]">{i + 1}.</span>{" "}
                {st.name}
                {i === fail && <span aria-label=" (fails here)"> ✕</span>}
              </button>
            </li>
          );
        })}
      </ol>
      <div aria-live="polite" className="flex flex-col gap-2">
        <p className="font-semibold text-fg-primary">
          {current.title ?? current.name}
        </p>
        <p className="text-fg-prose">{current.does}</p>
        {note !== undefined && (
          <div className={`admonition ${failing ? "caution" : "tip"}`}>
            <span className="adm-label">
              {failing ? (scenario.fail_label ?? "Denied") : "In this scenario"}
            </span>
            <p>{note}</p>
          </div>
        )}
        {fail > -1 && step > fail && (
          <div className="admonition warning">
            <p>Never reached: stopped at step {fail + 1}.</p>
          </div>
        )}
      </div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={step === 0}
          onClick={() => {
            setStep(Math.max(0, step - 1));
          }}
          className={pill(false)}
        >
          Back
        </button>
        <button
          type="button"
          disabled={step === data.steps.length - 1}
          onClick={() => {
            setStep(Math.min(data.steps.length - 1, step + 1));
          }}
          className={pill(false)}
        >
          Next
        </button>
        <span className="font-mono text-[12px] text-fg-tertiary">
          {step + 1} / {data.steps.length}
        </span>
      </div>
    </div>
  );
}

/** A toggle or nav button's classes, pressed or not. */
export function pill(on: boolean): string {
  return [
    "border px-2.5 py-1 text-[13px] disabled:opacity-60",
    on
      ? "border-accent bg-accent text-(--color-on-accent)"
      : "border-border-default bg-bg-raised text-fg-primary hover:bg-bg-hover",
  ].join(" ");
}
