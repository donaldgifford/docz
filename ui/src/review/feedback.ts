import type { Question as InspectQuestion } from "@/cli/__generated__/inspect";
import type { Annotation, DoczReview } from "@/cli/__generated__/review";
import type { Finding as ValidateFinding } from "@/cli/__generated__/validate";

/*
 * The text message a round sends the agent (DESIGN-0024 section 8,
 * INV-0027 Observation 7): one block per document, with the match
 * outcome on each anchored annotation. The closing instruction is what
 * makes the loop self-driving, so it is written only for a document
 * whose outcome is `changes`, and it names the questions that were
 * answered, never "each open question".
 */

export interface FeedbackDocument {
  record: DoczReview;
  title: string;
  /** The status the document had when the round opened. */
  currentStatus: string;
  /** `docz inspect`'s open questions, for titles and the unanswered ones. */
  questions: readonly InspectQuestion[];
  /** The round's findings, for the severity of a dismissed one. */
  findings: readonly Pick<ValidateFinding, "code" | "line" | "severity">[];
}

const OUTCOME: Record<DoczReview["outcome"], string> = {
  changes: "changes requested",
  approved: "approved",
  dismissed: "dismissed",
};

// A quote on one line, so a message never breaks inside an annotation.
const quote = (s: string) => `"${s.replace(/\s+/g, " ").trim()}"`;

function where(a: Annotation): string {
  const anchor = a.anchor;
  if (anchor === undefined) {
    return "[document]";
  }
  const place = [anchor.region, anchor.heading].filter((s) => s !== "");
  const lead = place.length > 0 ? `${place.join(" › ")}, ` : "";
  switch (anchor.resolution) {
    case "exact":
      return `[${lead}line ${String(anchor.line)}]`;
    case "ambiguous":
      return `[${lead}about line ${String(anchor.line)}; quote ambiguous]`;
    case "lost":
      return `[${lead}about line ${String(anchor.line)}; quote not found]`;
  }
}

function annotationLines(a: Annotation, n: number): string[] {
  const head = `${String(n)}. ${where(a)}`;
  const pad = " ".repeat(String(n).length + 2);
  const exact = quote(a.anchor?.exact ?? a.selection ?? "");
  switch (a.kind) {
    case "comment":
      return [`${head} ${exact}`, `${pad}comment: ${a.text ?? ""}`];
    case "delete":
      return [`${head} delete: ${exact}`];
    case "replace":
      return [
        `${head} replace ${exact}`,
        `${pad}→ ${quote(a.replacement ?? "")}`,
      ];
    case "document":
      return [`${head} ${a.text ?? ""}`];
    case "looks-good":
      return [];
  }
}

function plural(n: number, word: string): string {
  return `${String(n)} ${word}${n === 1 ? "" : "s"}`;
}

function noAction(doc: FeedbackDocument): string | undefined {
  const parts: string[] = [];
  const good = doc.record.annotations.filter((a) => a.kind === "looks-good");
  if (good.length > 0) {
    const places = good.map((a) => {
      const region = a.anchor?.region ?? "";
      const name = region !== "" ? region : (a.anchor?.heading ?? "");
      return `${name}, line ${String(a.anchor?.line ?? 0)}`;
    });
    parts.push(`${String(good.length)} "looks good" (${places.join("; ")})`);
  }
  const dismissed = doc.record.findings.filter((f) => f.action === "dismiss");
  for (const severity of ["error", "warning"] as const) {
    const these = dismissed.filter(
      (f) =>
        (doc.findings.find((r) => r.code === f.code && r.line === f.line)
          ?.severity ?? "warning") === severity,
    );
    if (these.length > 0) {
      const list = these
        .map((f) => `${f.code}, line ${String(f.line)}`)
        .join("; ");
      parts.push(`${plural(these.length, severity)} dismissed (${list})`);
    }
  }
  return parts.length > 0 ? `No action: ${parts.join("; ")}` : undefined;
}

function closing(doc: FeedbackDocument): string {
  const answered = doc.record.questions.map((q) => `Q${String(q.number)}`);
  const date = doc.record.submitted_at.slice(0, 10);
  const work: string[] = [];
  if (answered.length > 0) {
    const names =
      answered.length === 1
        ? (answered[0] ?? "")
        : `${answered.slice(0, -1).join(", ")} and ${answered.at(-1) ?? ""}`;
    work.push(
      `Resolve ${names} in the document in the fleet's format ` +
        `(\`> **Resolved ${date}: (a).**\`)`,
    );
  }
  const anchored = doc.record.annotations.some((a) => a.kind !== "looks-good");
  work.push(anchored ? "apply the annotations" : "apply the changes above");
  const sentence = work.join(", ");
  return `${sentence.charAt(0).toUpperCase()}${sentence.slice(1)}, then finish your turn so the next round can open.`;
}

/** One document's block of the message. */
export function documentFeedback(doc: FeedbackDocument): string {
  const { record } = doc;
  const out: string[] = [
    `docz review: ${record.document.id} ${doc.title}, round ${String(
      record.round,
    )}: ${OUTCOME[record.outcome]}`,
  ];
  if (record.outcome !== "changes") {
    return out.join("\n");
  }

  const open = doc.questions.filter((q) => q.resolved === null);
  if (open.length > 0) {
    out.push("", "Open questions:");
    for (const q of open) {
      const answer = record.questions.find((a) => a.number === q.number);
      const label = `- Q${String(q.number)} ${q.title}`;
      if (answer === undefined) {
        out.push(`- Q${String(q.number)} (no answer; leave open)`);
        continue;
      }
      const choice =
        answer.choice === "other"
          ? `other:\n  ${quote(answer.text ?? "")}`
          : answer.choice;
      out.push(`${label} → ${choice}`);
      if (answer.note !== undefined && answer.note !== "") {
        out.push(`  note: ${answer.note}`);
      }
    }
  }

  const acted = record.annotations.filter((a) => a.kind !== "looks-good");
  if (acted.length > 0) {
    out.push("", "Annotations:");
    acted.forEach((a, i) => out.push(...annotationLines(a, i + 1)));
  }

  const status: string[] = [];
  if (record.status !== doc.currentStatus) {
    status.push(`Status: set to ${record.status}`);
  }
  const fixes = record.findings.filter((f) => f.action === "fix");
  if (fixes.length > 0) {
    status.push(
      `Validation: ${fixes
        .map((f) => `fix ${f.code} at line ${String(f.line)}`)
        .join("; ")}`,
    );
  }
  if (status.length > 0) {
    out.push("", ...status);
  }

  const skipped = noAction(doc);
  if (skipped !== undefined) {
    out.push("", skipped);
  }
  out.push("", closing(doc));
  return out.join("\n");
}

/** The round's message, document by document. */
export function feedbackMessage(docs: readonly FeedbackDocument[]): string {
  return `${docs.map(documentFeedback).join("\n\n")}\n`;
}
