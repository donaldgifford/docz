import type {
  Annotation,
  DoczReview,
  Finding,
  Question,
} from "@/cli/__generated__/review";
import type { Resolved, SelectionAnchor } from "@/review/anchor";

/*
 * The review draft (DESIGN-0024 section 5): what the person has said
 * about one document in one round, before it is submitted. The page
 * edits it, the server autosaves it, and a submission turns it into the
 * record's questions, annotations, status, and findings.
 *
 * A submission's outcome comes from a defined actionable set: a
 * question answer, a comment, a replace, a delete, a document comment,
 * a status change, and a finding marked "fix". "Looks good" and a
 * dismissed finding are recorded as "no action" and never start a
 * round, and Approve is offered only while nothing actionable is
 * drafted, so approving never discards anything.
 */

export type AnnotationKind = Annotation["kind"];
export type FindingAction = Finding["action"];

export interface QuestionDraft {
  /** An option letter or "other"; unset while the card is unanswered. */
  choice?: string;
  /** The "Other" text. */
  text?: string;
  note?: string;
}

export interface AnnotationDraft {
  /** Stable within the draft, for editing and removal. */
  id: string;
  kind: AnnotationKind;
  /** Every kind but `document` is anchored to a selection. */
  anchor?: SelectionAnchor;
  text?: string;
  replacement?: string;
}

export interface FindingDraft {
  code: string;
  line: number;
  action: FindingAction;
}

export interface Draft {
  /** Keyed by question number; an absent entry is unanswered. */
  questions: Record<number, QuestionDraft>;
  annotations: AnnotationDraft[];
  /** The status the person chose; the document's own until changed. */
  status: string;
  findings: FindingDraft[];
}

export function emptyDraft(status: string): Draft {
  return { questions: {}, annotations: [], status, findings: [] };
}

/** A question counts as answered once it has a choice, and "other" text. */
export function isAnswered(q: QuestionDraft | undefined): boolean {
  if (q?.choice === undefined || q.choice === "") {
    return false;
  }
  return q.choice !== "other" || (q.text ?? "").trim() !== "";
}

const ACTIONABLE_KINDS: ReadonlySet<AnnotationKind> = new Set([
  "comment",
  "replace",
  "delete",
  "document",
]);

/** The actionable items in a draft, one entry per item, for display. */
export function actionableItems(draft: Draft, current: string): string[] {
  const items: string[] = [];
  for (const [n, q] of Object.entries(draft.questions)) {
    if (isAnswered(q)) {
      items.push(`question ${n}`);
    }
  }
  for (const a of draft.annotations) {
    if (ACTIONABLE_KINDS.has(a.kind)) {
      items.push(a.kind);
    }
  }
  if (draft.status !== current) {
    items.push("status");
  }
  for (const f of draft.findings) {
    if (f.action === "fix") {
      items.push(`fix ${f.code}`);
    }
  }
  return items;
}

/** Whether the draft holds anything that would start another round. */
export function hasActionable(draft: Draft, current: string): boolean {
  return actionableItems(draft, current).length > 0;
}

/** Approve is enabled only while the draft holds no actionable item. */
export function canApprove(draft: Draft, current: string): boolean {
  return !hasActionable(draft, current);
}

/**
 * Whether the draft holds nothing at all: no answered question, no
 * annotation of any kind, the document's own status, and no finding
 * acted on. Submit is disabled then, because it would record an
 * approval nobody chose; Approve is the way to say that.
 */
export function isEmptyDraft(draft: Draft, current: string): boolean {
  return (
    !Object.values(draft.questions).some(isAnswered) &&
    draft.annotations.length === 0 &&
    draft.status === current &&
    draft.findings.length === 0
  );
}

/** What a Submit of this draft records. */
export function outcomeOf(
  draft: Draft,
  current: string,
): DoczReview["outcome"] {
  return hasActionable(draft, current) ? "changes" : "approved";
}

/**
 * The annotations in document order: document comments first, then by
 * line and position in the line's text, draft order breaking ties.
 */
export function inDocumentOrder(
  annotations: readonly AnnotationDraft[],
): AnnotationDraft[] {
  const key = (a: AnnotationDraft) =>
    a.anchor === undefined ? -1 : a.anchor.line;
  return annotations
    .map((a, i) => ({ a, i }))
    .sort((x, y) => key(x.a) - key(y.a) || x.i - y.i)
    .map(({ a }) => a);
}

/** Resolves an anchor against the bytes the submission was made on. */
export type Resolver = (anchor: SelectionAnchor) => Resolved;

export interface RecordParts {
  outcome: DoczReview["outcome"];
  questions: Question[];
  annotations: Annotation[];
  status: string;
  findings: Finding[];
}

/**
 * The record's body for a Submit: answered questions only (an unanswered
 * card sends nothing), every annotation in document order with its
 * anchor resolved, the chosen status, and every finding acted on.
 */
export function toRecordParts(
  draft: Draft,
  current: string,
  resolve: Resolver,
): RecordParts {
  const questions: Question[] = Object.entries(draft.questions)
    .filter(([, q]) => isAnswered(q))
    .map(([n, q]) => ({
      number: Number(n),
      choice: q.choice ?? "",
      ...(q.choice === "other" ? { text: q.text ?? "" } : {}),
      ...(q.note !== undefined && q.note !== "" ? { note: q.note } : {}),
    }))
    .sort((a, b) => a.number - b.number);

  const annotations: Annotation[] = inDocumentOrder(draft.annotations).map(
    (a, i) => {
      const out: Annotation = { order: i + 1, kind: a.kind };
      if (a.anchor !== undefined) {
        const { region, heading, exact, prefix, suffix, selection } = a.anchor;
        const resolved = resolve(a.anchor);
        out.anchor = {
          region,
          ordinal: resolved.ordinal,
          heading,
          exact,
          prefix,
          suffix,
          line: resolved.line,
          resolution: resolved.resolution,
        };
        out.selection = selection;
      }
      if (a.text !== undefined && a.text !== "") {
        out.text = a.text;
      }
      if (a.kind === "replace") {
        out.replacement = a.replacement ?? "";
      }
      return out;
    },
  );

  return {
    outcome: outcomeOf(draft, current),
    questions,
    annotations,
    status: draft.status,
    findings: draft.findings.map(({ code, line, action }) => ({
      code,
      line,
      action,
    })),
  };
}
