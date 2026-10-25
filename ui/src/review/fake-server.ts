import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { DoczReview } from "@/cli/__generated__/review";
import type {
  DocumentState,
  ReviewDocument,
  ReviewSession,
  Submission,
  SubmissionKey,
} from "@/review/api";
import type { Draft } from "@/review/draft";

/*
 * An in-process stand-in for docz-review's local server, for vitest and
 * `just ui dev-review`. It speaks the same endpoints and enforces the
 * rules the page depends on: the bearer token, staleness by round and
 * digest, an idempotent submission id, a closed round, and 400 for a
 * choice, status, or finding the round did not offer. It writes no
 * files and runs no docz; documents arrive as `docz inspect` output.
 */

export const FAKE_ORIGIN = "http://127.0.0.1:4173";

interface Entry {
  served: ReviewDocument;
  /** What the file holds now; differs from `served` after `edit`. */
  current: DoczInspect;
}

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });

export interface FakeOptions {
  token: string;
  documents: ReviewDocument[];
  deadline?: string;
  now?: () => Date;
}

export class FakeReviewServer {
  readonly records: DoczReview[] = [];
  readonly drafts = new Map<string, Draft>();
  done = false;

  private readonly entries = new Map<string, Entry>();
  private readonly seen = new Map<string, DoczReview>();
  private readonly listeners = new Set<
    ReadableStreamDefaultController<string>
  >();
  private readonly token: string;
  private readonly deadline: string;
  private readonly now: () => Date;

  constructor(opts: FakeOptions) {
    this.token = opts.token;
    this.deadline = opts.deadline ?? "2026-10-25T16:00:00Z";
    this.now = opts.now ?? (() => new Date());
    for (const doc of opts.documents) {
      this.entries.set(doc.key, { served: doc, current: doc.inspect });
    }
  }

  /** A `fetch` that answers from this server. */
  readonly fetch = (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> =>
    this.handle(
      new Request(
        typeof input === "string" ? new URL(input, FAKE_ORIGIN) : input,
        init,
      ),
    );

  /** The file changes on disk: submissions against the old bytes are stale. */
  edit(key: string, next: DoczInspect): void {
    const entry = this.entries.get(key);
    if (entry !== undefined) {
      entry.current = next;
    }
  }

  /** Opens the next round for a document, as the agent's next Stop would. */
  nextRound(key: string, next: DoczInspect): void {
    const entry = this.entries.get(key);
    if (entry === undefined) {
      return;
    }
    const n = entry.served.round_number + 1;
    entry.served = {
      ...entry.served,
      round: `round-${key}-${String(n)}`,
      round_number: n,
      state: "open",
      previous: entry.served.inspect.content,
      inspect: next,
      draft: null,
    };
    entry.current = next;
    this.notify("updated", "");
  }

  async handle(req: Request): Promise<Response> {
    if (req.headers.get("Authorization") !== `Bearer ${this.token}`) {
      return json(401, { error: "unauthorized" });
    }
    const { pathname } = new URL(req.url);
    if (req.method === "GET" && pathname === "/api/review") {
      return json(200, this.session());
    }
    if (req.method === "GET" && pathname === "/api/events") {
      return this.events();
    }
    if (req.method === "POST" && pathname === "/api/review/done") {
      this.done = true;
      this.notify("closed", "done");
      return json(200, {});
    }
    const match =
      /^\/api\/review\/([^/]+)\/(submit|approve|dismiss|draft)$/.exec(pathname);
    const entry =
      match?.[1] === undefined
        ? undefined
        : this.entries.get(decodeURIComponent(match[1]));
    if (match === null || entry === undefined) {
      return json(404, { error: "not found" });
    }
    const body = (await req.json()) as Record<string, unknown>;
    const action = match[2];
    if (action === "draft" && req.method === "PUT") {
      if (body.round !== entry.served.round) {
        return json(409, { error: "the round has moved on" });
      }
      this.drafts.set(entry.served.key, body.draft as Draft);
      return json(200, {});
    }
    if (req.method !== "POST") {
      return json(405, { error: "method not allowed" });
    }
    return this.record(entry, action ?? "", body);
  }

  private session(): ReviewSession {
    return {
      deadline: this.deadline,
      documents: [...this.entries.values()].map((e) => ({
        ...e.served,
        draft: this.drafts.get(e.served.key) ?? e.served.draft,
      })),
    };
  }

  private record(
    entry: Entry,
    action: string,
    body: Record<string, unknown>,
  ): Response {
    const key = body as unknown as SubmissionKey;
    const again = this.seen.get(key.submission_id);
    if (again !== undefined) {
      return json(200, { outcome: again.outcome, state: entry.served.state });
    }
    if (key.round !== entry.served.round || entry.served.state !== "open") {
      return json(409, { error: "the round is closed" });
    }
    if (key.content_sha256 !== entry.current.content_sha256) {
      return json(409, { error: "the document changed since the page loaded" });
    }

    const inspect = entry.served.inspect;
    const sub = body as unknown as Submission;
    const parts =
      action === "submit"
        ? {
            outcome: sub.outcome,
            questions: sub.questions,
            annotations: sub.annotations,
            status: sub.status,
            findings: sub.findings,
          }
        : {
            outcome: action === "approve" ? "approved" : "dismissed",
            questions: [],
            annotations: [],
            status: inspect.frontmatter?.status ?? "",
            findings: [],
          };
    const refused = this.refuse(entry, parts);
    if (refused !== undefined) {
      return json(400, { error: refused });
    }

    const record: DoczReview = {
      schema_version: 1,
      review_id: `r_${entry.served.key}`,
      round: entry.served.round_number,
      submission_id: key.submission_id,
      submitted_at: this.now()
        .toISOString()
        .replace(/\.\d+Z$/, "Z"),
      document: {
        id: inspect.id,
        path: inspect.path,
        type: inspect.type,
        content_sha256: inspect.content_sha256,
      },
      ...(parts as Pick<
        DoczReview,
        "outcome" | "questions" | "annotations" | "status" | "findings"
      >),
    };
    this.records.push(record);
    this.seen.set(key.submission_id, record);
    const state: DocumentState =
      record.outcome === "changes" ? "changes" : record.outcome;
    entry.served = { ...entry.served, state };
    this.drafts.delete(entry.served.key);
    return json(200, { outcome: record.outcome, state });
  }

  // The round's own offer is the only valid answer set.
  private refuse(
    entry: Entry,
    parts: Pick<DoczReview, "questions" | "status" | "findings">,
  ): string | undefined {
    const inspect = entry.served.inspect;
    for (const q of parts.questions) {
      const offered = inspect.open_questions.find((o) => o.number === q.number);
      const letters = offered?.options.map((o) => o.letter) ?? [];
      if (
        offered === undefined ||
        (q.choice !== "other" && !letters.includes(q.choice))
      ) {
        return `question ${String(q.number)} has no option ${q.choice}`;
      }
    }
    if (parts.status !== "" && !inspect.statuses.includes(parts.status)) {
      return `status ${parts.status} is not allowed`;
    }
    for (const f of parts.findings) {
      if (
        !entry.served.findings.some(
          (r) => r.code === f.code && r.line === f.line,
        )
      ) {
        return `finding ${f.code} at line ${String(f.line)} was not reported`;
      }
    }
    return undefined;
  }

  private events(): Response {
    let controller: ReadableStreamDefaultController<string> | undefined;
    const stream = new ReadableStream<string>({
      start: (c) => {
        controller = c;
        this.listeners.add(c);
        c.enqueue(": connected\n\n");
      },
      cancel: () => {
        if (controller !== undefined) {
          this.listeners.delete(controller);
        }
      },
    });
    return new Response(stream.pipeThrough(new TextEncoderStream()), {
      headers: { "Content-Type": "text/event-stream" },
    });
  }

  private notify(event: string, data: string): void {
    for (const c of this.listeners) {
      c.enqueue(`event: ${event}\ndata: ${data}\n\n`);
    }
  }
}
