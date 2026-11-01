import { useId, useState } from "react";

import { pill } from "@/markdown/widgets/stepper";

import type {
  ScoreCheck,
  ScorecardData,
  ScoreResult,
} from "@/markdown/widgets/validate";

/*
 * docz-scorecard: the checks, and a score for results the reader pastes
 * as JSON or loads from the block's example. The arithmetic is the
 * export runtime's (runtime.js score): a check's weight defaults to 1,
 * only a score between 0 and 1 counts as measured, and the total is the
 * measured points over the measured weights. Pasted results stay in
 * this page; nothing is stored.
 */

interface Row {
  check: ScoreCheck;
  weight: number;
  result: Partial<ScoreResult>;
  measured: boolean;
}

interface Scored {
  rows: Row[];
  points: number;
  weights: number;
}

interface Results {
  subject?: unknown;
  generated_at?: unknown;
  results: unknown[];
}

export function score(
  checks: readonly ScoreCheck[],
  results: unknown[],
): Scored {
  const byId = new Map<string, Partial<ScoreResult>>();
  for (const r of results) {
    if (r !== null && typeof r === "object") {
      const res = r as Partial<ScoreResult>;
      if (typeof res.id === "string" && res.id !== "") {
        byId.set(res.id, res);
      }
    }
  }
  let points = 0;
  let weights = 0;
  const rows = checks.map((check): Row => {
    const weight =
      check.weight !== undefined && check.weight > 0 ? check.weight : 1;
    const result = byId.get(check.id) ?? {};
    const s = result.score;
    const measured = typeof s === "number" && s >= 0 && s <= 1;
    if (measured) {
      points += s * weight;
      weights += weight;
    }
    return { check, weight, result, measured };
  });
  return { rows, points, weights };
}

type Outcome =
  | { kind: "none" }
  | { kind: "error"; message: string }
  | { kind: "scored"; results: Results; label: string };

function parse(text: string): Outcome {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (e) {
    return {
      kind: "error",
      message: `Not valid JSON: ${e instanceof Error ? e.message : String(e)}`,
    };
  }
  if (
    parsed === null ||
    typeof parsed !== "object" ||
    !Array.isArray((parsed as { results?: unknown }).results)
  ) {
    return { kind: "error", message: "The JSON needs a results list." };
  }
  return { kind: "scored", results: parsed as Results, label: "pasted" };
}

export function Scorecard({ data }: { data: ScorecardData }) {
  const [text, setText] = useState("");
  const [outcome, setOutcome] = useState<Outcome>({ kind: "none" });
  const inputId = useId();
  const first = data.checks[0]?.id ?? "C1";

  return (
    <div className="flex flex-col gap-3 text-[14px]">
      <label htmlFor={inputId} className="text-[13px] text-fg-tertiary">
        Results as JSON
      </label>
      <textarea
        id={inputId}
        rows={6}
        spellCheck={false}
        value={text}
        placeholder={`{"results": [{"id": "${first}", "score": 1}]}`}
        onChange={(e) => {
          setText(e.target.value);
        }}
        className="border border-border-default bg-bg-raised p-2 font-mono text-[13px] text-fg-primary"
      />
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className={pill(false)}
          onClick={() => {
            setOutcome(parse(text));
          }}
        >
          Score
        </button>
        {data.example !== undefined && (
          <button
            type="button"
            className={pill(false)}
            onClick={() => {
              const example = data.example;
              if (example === undefined) {
                return;
              }
              setText(JSON.stringify(example, null, 2));
              setOutcome({
                kind: "scored",
                results: example,
                label: "example, not real results",
              });
            }}
          >
            Load the example
          </button>
        )}
        <button
          type="button"
          className={pill(false)}
          onClick={() => {
            setText("");
            setOutcome({ kind: "none" });
          }}
        >
          Clear
        </button>
      </div>
      <div aria-live="polite">
        {outcome.kind === "error" && (
          <div className="admonition caution">
            <p>{outcome.message}</p>
          </div>
        )}
        {outcome.kind === "scored" && (
          <ScoreTable
            checks={data.checks}
            note={data.note}
            results={outcome.results}
            label={outcome.label}
          />
        )}
      </div>
    </div>
  );
}

function ScoreTable({
  checks,
  note,
  results,
  label,
}: {
  checks: readonly ScoreCheck[];
  note: string | undefined;
  results: Results;
  label: string;
}) {
  const sc = score(checks, results.results);
  const pct = sc.weights > 0 ? Math.round((sc.points / sc.weights) * 100) : 0;
  const measured = sc.rows.filter((r) => r.measured).length;
  const tags = [label, results.subject, results.generated_at].filter(
    (t): t is string => typeof t === "string" && t !== "",
  );

  return (
    <div className="flex flex-col gap-3">
      <div className="border border-border-default bg-bg-raised p-3">
        <div className="flex flex-wrap items-baseline gap-2">
          <p className="text-[18px] font-semibold text-fg-primary">
            {sc.points.toFixed(1)} / {sc.weights.toFixed(1)}
          </p>
          {tags.map((t) => (
            <span
              key={t}
              className="border border-border-default px-1.5 font-mono text-[11px] text-fg-secondary"
            >
              {t}
            </span>
          ))}
        </div>
        <div
          role="meter"
          aria-label="Score"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct}
          className="mt-2 h-1.5 bg-bg-elevated"
        >
          <div
            className="h-full bg-accent"
            style={{ width: `${String(pct)}%` }}
          />
        </div>
        <p className="mt-2 text-[13px] text-fg-tertiary">
          {pct}% of the target, measured on {measured} of {checks.length}{" "}
          checks.{note === undefined ? "" : ` ${note}`}
        </p>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {["#", "Check", "Observed", "Target", "Points", "Fixes it"].map(
                (h) => (
                  <th key={h}>{h}</th>
                ),
              )}
            </tr>
          </thead>
          <tbody>
            {sc.rows.map((row) => {
              const s = row.result.score;
              return (
                <tr key={row.check.id}>
                  <td>{row.check.id}</td>
                  <td>
                    <b>{row.check.name}</b>
                    {row.check.source !== undefined && (
                      <span className="ml-1 font-mono text-[11px] text-fg-tertiary">
                        {row.check.source}
                      </span>
                    )}
                  </td>
                  <td>
                    {typeof row.result.observed === "string"
                      ? row.result.observed
                      : ""}
                    {typeof row.result.evidence === "string" && (
                      <span className="block text-[12px] text-fg-tertiary">
                        {row.result.evidence}
                      </span>
                    )}
                  </td>
                  <td>{row.check.target}</td>
                  <td>
                    {row.measured && typeof s === "number"
                      ? `${(s * row.weight).toFixed(1)} of ${String(row.weight)}`
                      : "not measured"}
                  </td>
                  <td>{row.check.fixes ?? ""}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
