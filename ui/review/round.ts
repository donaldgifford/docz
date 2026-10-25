/*
 * One review run's rounds (DESIGN-0024 sections 6 and 8): the documents a
 * server opened, each with a round id, the snapshot it was opened from,
 * and the previous round's source for the diff.
 *
 * Every submission, Approve and Dismiss included, is checked in this
 * order before anything is written:
 *
 *   1. a submission id already recorded is acknowledged again, once;
 *   2. a round that is not the one served, or is already closed, is 409;
 *   3. the file is re-read through docz: a different id, a different
 *      digest, or a path that no longer resolves is 409 (stale), and the
 *      page keeps its draft;
 *   4. question choices, the status, and finding identities must be ones
 *      the round offered, else 400;
 *   5. the record must validate against review.schema.json, else 400.
 *
 * Only then is the record written and fsynced, the state moved under the
 * lock, and the answer sent, so a reply always means a record on disk.
 */
import { randomBytes } from "node:crypto";

import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { DoczReview } from "@/cli/__generated__/review";
import type { Finding } from "@/cli/__generated__/validate";
import { ContractError, parseReview } from "@/cli/validate";
import type {
  DocumentState,
  ReviewDocument,
  ReviewSession,
  Submission,
  SubmissionKey,
} from "@/review/api";
import type { Draft } from "@/review/draft";
import { type FeedbackDocument, feedbackMessage } from "@/review/feedback";

import type { Docz } from "./docz";
import type { Entry, Session } from "./state";

export type Action = "submit" | "approve" | "dismiss";

/** An HTTP-shaped answer, so the server only serialises it. */
export interface Reply {
  status: number;
  body: unknown;
}

interface RoundDoc {
  entry: Entry;
  round: string;
  inspect: DoczInspect;
  findings: Finding[];
  previous: string | null;
  draft: Draft | null;
  state: DocumentState;
  record: DoczReview | null;
}

export interface OpenOptions {
  session: Session;
  docz: Docz;
  /** The record's review_id: the session id, or a review by hand's id. */
  reviewId: string;
  deadline: Date;
  /**
   * A document that has had this many rounds since it was last armed is
   * dismissed instead of opened.
   */
  maxRounds: number;
  now?: () => Date;
}

/** What opening did with each entry it was given. */
export interface Opened {
  review: Review;
  /** Entries dismissed by the round limit. */
  limited: Entry[];
  /** Entries docz could not read, with the reason; left as they were. */
  failed: { entry: Entry; reason: string }[];
}

const roundId = () => `rd_${randomBytes(12).toString("hex")}`;

const ok = (body: unknown): Reply => ({ status: 200, body });
const fail = (status: number, error: string): Reply => ({
  status,
  body: { error },
});

export class Review {
  private readonly docs = new Map<string, RoundDoc>();
  private readonly seen = new Map<string, DoczReview>();
  private readonly queue = new Map<string, Promise<unknown>>();
  private readonly now: () => Date;

  private constructor(private readonly opts: OpenOptions) {
    this.now = opts.now ?? (() => new Date());
  }

  /**
   * Opens a round for each entry: the round limit first, then a snapshot
   * through docz, then the state moves to open and the snapshot is kept
   * for the next round's diff.
   */
  static async open(
    entries: readonly Entry[],
    opts: OpenOptions,
  ): Promise<Opened> {
    const review = new Review(opts);
    const limited: Entry[] = [];
    const failed: Opened["failed"] = [];
    for (const entry of entries) {
      if (
        entry.state !== "open" &&
        entry.round - entry.armed_round >= opts.maxRounds
      ) {
        limited.push(opts.session.apply(entry.key, "dismissed"));
        continue;
      }
      let snap;
      try {
        snap = await opts.docz.snapshot(entry.path, entry);
      } catch (err) {
        failed.push({ entry, reason: (err as Error).message });
        continue;
      }
      const opened = opts.session.apply(entry.key, "open", {
        digest: snap.inspect.content_sha256,
      });
      opts.session.writeSnapshot(
        opened.key,
        opened.round,
        snap.inspect.content,
      );
      review.docs.set(opened.id, {
        entry: opened,
        round: roundId(),
        inspect: snap.inspect,
        findings: snap.findings,
        previous: opts.session.readSnapshot(opened.key, opened.round - 1),
        draft: opts.session.readDraft(opened.key, opened.round) as Draft | null,
        state: "open",
        record: null,
      });
    }
    return { review, limited, failed };
  }

  get size(): number {
    return this.docs.size;
  }

  /** Every document has an outcome. */
  settled(): boolean {
    return [...this.docs.values()].every((d) => d.state !== "open");
  }

  /** The records written in this run, in document order. */
  records(): DoczReview[] {
    return [...this.docs.values()].flatMap((d) =>
      d.record === null ? [] : [d.record],
    );
  }

  /** `GET /api/review`. */
  view(): ReviewSession {
    return {
      deadline: this.opts.deadline.toISOString(),
      documents: [...this.docs.values()].map((d): ReviewDocument => ({
        key: d.entry.id,
        round: d.round,
        round_number: d.entry.round,
        state: d.state,
        inspect: d.inspect,
        findings: d.findings,
        previous: d.previous,
        draft: d.draft,
      })),
    };
  }

  /** `PUT /api/review/{doc}/draft`. */
  saveDraft(key: string, body: unknown): Reply {
    const doc = this.docs.get(key);
    if (doc === undefined) {
      return fail(404, "not found");
    }
    const { round, draft } = (body ?? {}) as {
      round?: unknown;
      draft?: unknown;
    };
    if (round !== doc.round || doc.state !== "open") {
      return fail(409, "the round is closed");
    }
    if (draft === null || typeof draft !== "object") {
      return fail(400, "draft must be an object");
    }
    doc.draft = draft as Draft;
    this.opts.session.writeDraft(doc.entry.key, doc.entry.round, draft);
    return ok({});
  }

  /** `POST /api/review/{doc}/{submit|approve|dismiss}`, one at a time per document. */
  record(key: string, action: Action, body: unknown): Promise<Reply> {
    const prev = this.queue.get(key) ?? Promise.resolve();
    const next = prev.then(() => this.recordNow(key, action, body));
    this.queue.set(
      key,
      next.catch(() => undefined),
    );
    return next;
  }

  /**
   * Done, the deadline, an expired grace period, or a signal: every
   * document still open is dismissed, with no record, since nobody said
   * anything about it.
   */
  dismissRest(): void {
    for (const doc of this.docs.values()) {
      if (doc.state === "open") {
        this.opts.session.apply(doc.entry.key, "dismissed");
        doc.state = "dismissed";
      }
    }
  }

  /**
   * The hook's message: one block per document that ended in changes,
   * or null when none did and the turn may end.
   */
  feedback(): string | null {
    const docs: FeedbackDocument[] = [...this.docs.values()].flatMap((d) =>
      d.record?.outcome === "changes"
        ? [
            {
              record: d.record,
              title: d.inspect.frontmatter?.title ?? d.entry.id,
              currentStatus: d.inspect.frontmatter?.status ?? "",
              questions: d.inspect.open_questions,
              findings: d.findings,
            },
          ]
        : [],
    );
    return docs.length === 0 ? null : feedbackMessage(docs);
  }

  private async recordNow(
    key: string,
    action: Action,
    body: unknown,
  ): Promise<Reply> {
    const doc = this.docs.get(key);
    if (doc === undefined) {
      return fail(404, "not found");
    }
    const sub = (body ?? {}) as Partial<Submission>;
    if (typeof sub.submission_id !== "string" || sub.submission_id === "") {
      return fail(400, "submission_id is required");
    }
    const again = this.seen.get(sub.submission_id);
    if (again !== undefined) {
      return ok({ outcome: again.outcome, state: doc.state });
    }
    if (sub.round !== doc.round || doc.state !== "open") {
      return fail(409, "the round is closed");
    }

    const stale = await this.staleness(doc, sub as SubmissionKey);
    if (stale !== null) {
      return fail(409, stale);
    }

    const parts =
      action === "submit"
        ? {
            outcome: sub.outcome,
            questions: sub.questions ?? [],
            annotations: sub.annotations ?? [],
            status: sub.status ?? "",
            findings: sub.findings ?? [],
          }
        : {
            outcome:
              action === "approve"
                ? ("approved" as const)
                : ("dismissed" as const),
            questions: [],
            annotations: [],
            status: "",
            findings: [],
          };
    const refused = refuse(doc, parts);
    if (refused !== null) {
      return fail(400, refused);
    }

    let record: DoczReview;
    try {
      record = parseReview(
        JSON.stringify({
          schema_version: 1,
          review_id: this.opts.reviewId,
          round: doc.entry.round,
          submission_id: sub.submission_id,
          submitted_at: this.now()
            .toISOString()
            .replace(/\.\d+Z$/, "Z"),
          document: {
            id: doc.inspect.id,
            path: doc.inspect.path,
            type: doc.inspect.type,
            content_sha256: doc.inspect.content_sha256,
          },
          ...parts,
        }),
      );
    } catch (err) {
      if (err instanceof ContractError) {
        return fail(400, err.message);
      }
      throw err;
    }

    const session = this.opts.session;
    session.writeRecord(doc.entry.key, doc.entry.round, record);
    doc.entry = session.apply(doc.entry.key, record.outcome, {
      submission_id: record.submission_id,
    });
    session.deleteDraft(doc.entry.key, doc.entry.round);
    doc.state = record.outcome;
    doc.record = record;
    doc.draft = null;
    this.seen.set(record.submission_id, record);
    return ok({ outcome: record.outcome, state: doc.state });
  }

  /** Why the bytes on disk are not the bytes the page showed, or null. */
  private async staleness(
    doc: RoundDoc,
    key: SubmissionKey,
  ): Promise<string | null> {
    if (key.content_sha256 !== doc.inspect.content_sha256) {
      return "the submission was made against other bytes than this round's";
    }
    let now: DoczInspect;
    try {
      now = await this.opts.docz.inspect(doc.entry.path, doc.entry);
    } catch (err) {
      return `the document no longer resolves: ${(err as Error).message}`;
    }
    if (now.id !== doc.inspect.id) {
      return `${doc.entry.path} is now ${now.id}, not ${doc.inspect.id}`;
    }
    if (now.content_sha256 !== doc.inspect.content_sha256) {
      return "the document changed since the page loaded";
    }
    return null;
  }
}

/** The round's own offer is the only valid answer set. */
function refuse(
  doc: RoundDoc,
  parts: {
    outcome: unknown;
    questions: DoczReview["questions"];
    status: string;
    findings: DoczReview["findings"];
  },
): string | null {
  if (
    parts.outcome !== "changes" &&
    parts.outcome !== "approved" &&
    parts.outcome !== "dismissed"
  ) {
    return "outcome must be changes, approved, or dismissed";
  }
  for (const q of parts.questions) {
    const offered = doc.inspect.open_questions.find(
      (o) => o.number === q.number,
    );
    const letters = offered?.options.map((o) => o.letter) ?? [];
    if (
      offered === undefined ||
      (q.choice !== "other" && !letters.includes(q.choice))
    ) {
      return `question ${String(q.number)} has no option ${q.choice}`;
    }
  }
  if (parts.status !== "" && !doc.inspect.statuses.includes(parts.status)) {
    return `status ${parts.status} is not allowed`;
  }
  for (const f of parts.findings) {
    if (!doc.findings.some((r) => r.code === f.code && r.line === f.line)) {
      return `finding ${f.code} at line ${String(f.line)} was not reported`;
    }
  }
  return null;
}
