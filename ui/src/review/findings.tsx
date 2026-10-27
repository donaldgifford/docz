import { ReviewButton } from "@/review/controls";

import type { Finding } from "@/cli/__generated__/validate";
import type { FindingAction, FindingDraft } from "@/review/draft";

/*
 * `docz validate`'s findings for the bytes on the page, shown above the
 * document (DESIGN-0024 section 4): an error red, a warning amber, each
 * with "dismiss" (recorded, not actionable) and "fix this" (actionable).
 * A finding is identified by its code and line, which is how the server
 * checks it against the round it served.
 */

const same = (a: { code: string; line: number }, b: typeof a) =>
  a.code === b.code && a.line === b.line;

/** Sets, changes, or (with the same action again) clears a finding's action. */
export function toggleFinding(
  drafts: readonly FindingDraft[],
  finding: Pick<Finding, "code" | "line">,
  action: FindingAction,
): FindingDraft[] {
  const current = drafts.find((d) => same(d, finding));
  const rest = drafts.filter((d) => !same(d, finding));
  return current?.action === action
    ? rest
    : [...rest, { code: finding.code, line: finding.line, action }];
}

export function Findings({
  findings,
  actions,
  onChange,
}: {
  findings: readonly Finding[];
  actions: readonly FindingDraft[];
  onChange: (next: FindingDraft[]) => void;
}) {
  if (findings.length === 0) {
    return (
      <p className="font-mono text-[12.5px] text-fg-tertiary">
        docz validate reported nothing for this document.
      </p>
    );
  }
  return (
    <ul aria-label="Findings" className="flex flex-col gap-1">
      {findings.map((f) => {
        const action = actions.find((a) => same(a, f))?.action;
        const tone =
          f.severity === "error"
            ? "border-st-rejected text-st-rejected"
            : "border-st-draft text-st-draft";
        return (
          <li
            key={`${f.code}:${String(f.line)}:${f.detail}`}
            data-finding={f.code}
            data-severity={f.severity}
            className={`flex flex-wrap items-baseline gap-2 border-l-2 bg-bg-raised px-2 py-1 ${tone}`}
          >
            <span className="font-mono text-[12px] uppercase">
              {f.severity}
            </span>
            <code className="font-mono text-[12.5px] text-fg-primary">
              {f.code}
            </code>
            {f.line > 0 && (
              <span className="font-mono text-[12px] text-fg-tertiary">
                line {f.line}
              </span>
            )}
            <span className="text-[13px] text-fg-secondary">{f.detail}</span>
            <span className="ml-auto flex gap-1">
              <ReviewButton
                aria-pressed={action === "dismiss"}
                variant={action === "dismiss" ? "selected" : "plain"}
                onClick={() => {
                  onChange(toggleFinding(actions, f, "dismiss"));
                }}
              >
                Dismiss
              </ReviewButton>
              <ReviewButton
                aria-pressed={action === "fix"}
                variant={action === "fix" ? "selected" : "plain"}
                onClick={() => {
                  onChange(toggleFinding(actions, f, "fix"));
                }}
              >
                Fix this
              </ReviewButton>
            </span>
          </li>
        );
      })}
    </ul>
  );
}
