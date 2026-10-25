import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { DoczReview } from "@/cli/__generated__/review";
import type { Finding } from "@/cli/__generated__/validate";
import type { Draft } from "@/review/draft";

/*
 * The review page's client for docz-review's local server (DESIGN-0024
 * section 6). The server's one credential is a random token in the
 * bootstrap URL: `takeToken` reads it once, removes it from the location
 * bar with `history.replaceState`, and the client keeps it in memory and
 * sends it as `Authorization: Bearer` on every call. No cookie is ever
 * set, so nothing ambient reaches another loopback port. EventSource
 * cannot carry a header, so the event stream is read through fetch.
 */

/** A document's place in the session's state machine. */
export type DocumentState = "open" | "changes" | "approved" | "dismissed";

/** One document in the current round, as `GET /api/review` serves it. */
export interface ReviewDocument {
  /** The `{doc}` segment of the document's endpoints: its id. */
  key: string;
  /** The round id every submission must carry. */
  round: string;
  /** 1 for the first round. */
  round_number: number;
  state: DocumentState;
  /** `docz inspect` for the bytes on the page; `content_sha256` is the digest. */
  inspect: DoczInspect;
  /** `docz validate` for the same bytes. */
  findings: Finding[];
  /** The previous round's source, for the diff; null in round one. */
  previous: string | null;
  /** The autosaved draft, if any. */
  draft: Draft | null;
}

export interface ReviewSession {
  documents: ReviewDocument[];
  /** When the server gives up waiting, RFC 3339. */
  deadline: string;
}

/** Every mutation names the bytes and round it was made against. */
export interface SubmissionKey {
  round: string;
  content_sha256: string;
  submission_id: string;
}

export type Submission = SubmissionKey &
  Pick<
    DoczReview,
    "outcome" | "questions" | "annotations" | "status" | "findings"
  >;

export interface SubmitResult {
  outcome: DoczReview["outcome"];
  state: DocumentState;
}

/** The file changed under the page: review the new bytes first. */
export class StaleError extends Error {
  constructor(readonly detail: string) {
    super(`stale: ${detail}`);
    this.name = "StaleError";
  }
}

/** The server refused the request: a bad token or a bad submission. */
export class ReviewApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ReviewApiError";
  }
}

export type ReviewEvent =
  { type: "updated" } | { type: "closed"; reason: string };

/**
 * Reads the bootstrap token from `?t=` and removes it from the location,
 * so the credential is in memory only. Null when the URL carries none.
 */
export function takeToken(loc: Location = window.location): string | null {
  const url = new URL(loc.href);
  const token = url.searchParams.get("t");
  if (token === null || token === "") {
    return null;
  }
  url.searchParams.delete("t");
  window.history.replaceState(
    window.history.state,
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
  return token;
}

/** A random id for a submission, so a retry is acknowledged, not doubled. */
export function newSubmissionId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(12));
  return `s_${Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("")}`;
}

async function errorText(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: unknown };
    return typeof body.error === "string" ? body.error : res.statusText;
  } catch {
    return res.statusText;
  }
}

export interface ReviewClient {
  session(): Promise<ReviewSession>;
  submit(doc: string, body: Submission): Promise<SubmitResult>;
  approve(doc: string, key: SubmissionKey): Promise<SubmitResult>;
  dismiss(doc: string, key: SubmissionKey): Promise<SubmitResult>;
  done(): Promise<void>;
  saveDraft(doc: string, round: string, draft: Draft): Promise<void>;
  events(onEvent: (e: ReviewEvent) => void, signal: AbortSignal): Promise<void>;
}

/** The part of `fetch` the client calls, so a fake need not be all of it. */
export type FetchLike = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export function createReviewClient(
  token: string,
  fetchImpl: FetchLike = (input, init) => fetch(input, init),
): ReviewClient {
  const call = async (
    method: string,
    path: string,
    body?: unknown,
    signal?: AbortSignal,
  ): Promise<Response> => {
    const headers: Record<string, string> = {
      Authorization: `Bearer ${token}`,
    };
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
    }
    const res = await fetchImpl(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
      credentials: "omit",
    });
    if (res.status === 409) {
      throw new StaleError(await errorText(res));
    }
    if (!res.ok) {
      throw new ReviewApiError(res.status, await errorText(res));
    }
    return res;
  };
  const docPath = (doc: string, action: string) =>
    `/api/review/${encodeURIComponent(doc)}/${action}`;

  return {
    async session() {
      return (await (await call("GET", "/api/review")).json()) as ReviewSession;
    },
    async submit(doc, body) {
      const res = await call("POST", docPath(doc, "submit"), body);
      return (await res.json()) as SubmitResult;
    },
    async approve(doc, key) {
      const res = await call("POST", docPath(doc, "approve"), key);
      return (await res.json()) as SubmitResult;
    },
    async dismiss(doc, key) {
      const res = await call("POST", docPath(doc, "dismiss"), key);
      return (await res.json()) as SubmitResult;
    },
    async done() {
      await call("POST", "/api/review/done", {});
    },
    async saveDraft(doc, round, draft) {
      await call("PUT", docPath(doc, "draft"), { round, draft });
    },
    async events(onEvent, signal) {
      const res = await call("GET", "/api/events", undefined, signal);
      if (res.body === null) {
        return;
      }
      const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
      // A fetch that ignores the signal still stops reading on abort.
      signal.addEventListener("abort", () => {
        reader.cancel().catch(() => undefined);
      });
      let buffer = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) {
          return;
        }
        buffer += value;
        let cut = buffer.indexOf("\n\n");
        while (cut !== -1) {
          const event = parseEvent(buffer.slice(0, cut));
          buffer = buffer.slice(cut + 2);
          if (event !== null) {
            onEvent(event);
          }
          cut = buffer.indexOf("\n\n");
        }
      }
    },
  };
}

/** One server-sent event block; comments and unknown events are null. */
export function parseEvent(block: string): ReviewEvent | null {
  let name = "message";
  const data: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) {
      name = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      data.push(line.slice(5).trimStart());
    }
  }
  if (name === "updated") {
    return { type: "updated" };
  }
  if (name === "closed") {
    return { type: "closed", reason: data.join("\n") };
  }
  return null;
}
