import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import {
  newSubmissionId,
  StaleError,
  type ReviewClient,
  type ReviewDocument,
} from "@/review/api";
import { ReviewButton } from "@/review/controls";
import { DocumentReview } from "@/review/document-review";
import { emptyDraft, type Draft } from "@/review/draft";

/*
 * The review page (DESIGN-0024 section 5): a switcher over the round's
 * documents, one DocumentReview at a time, and Done. Drafts live here,
 * keyed by document, so a stale submission or a reload keeps them; each
 * is autosaved to the server, and the page warns before unload while
 * any draft holds something.
 */

export const SESSION_KEY = ["review-session"] as const;
const AUTOSAVE_MS = 800;

function isEmpty(draft: Draft, current: string): boolean {
  return (
    Object.keys(draft.questions).length === 0 &&
    draft.annotations.length === 0 &&
    draft.findings.length === 0 &&
    draft.status === current
  );
}

function without<T>(all: Record<string, T>, key: string): Record<string, T> {
  return Object.fromEntries(Object.entries(all).filter(([k]) => k !== key));
}

const statusOf = (doc: ReviewDocument) => doc.inspect.frontmatter?.status ?? "";

export function ReviewApp({ client }: { client: ReviewClient }) {
  const queryClient = useQueryClient();
  const session = useQuery({
    queryKey: SESSION_KEY,
    queryFn: () => client.session(),
  });
  const [active, setActive] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [stale, setStale] = useState<Record<string, string>>({});
  const [closed, setClosed] = useState<string | null>(null);
  const timers = useRef(new Map<string, number>());

  const documents = session.data?.documents ?? [];
  const doc = documents.find((d) => d.key === active) ?? documents[0];
  const draftFor = (d: ReviewDocument): Draft =>
    drafts[d.key] ?? d.draft ?? emptyDraft(statusOf(d));

  // The server's events: a new round, or the session ending.
  useEffect(() => {
    const abort = new AbortController();
    client
      .events((event) => {
        if (event.type === "updated") {
          void queryClient.invalidateQueries({ queryKey: SESSION_KEY });
        } else {
          setClosed(event.reason);
        }
      }, abort.signal)
      .catch(() => undefined);
    return () => {
      abort.abort();
    };
  }, [client, queryClient]);

  const dirty = documents.some(
    (d) => d.state === "open" && !isEmpty(draftFor(d), statusOf(d)),
  );
  useEffect(() => {
    if (!dirty) {
      return;
    }
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", warn);
    return () => {
      window.removeEventListener("beforeunload", warn);
    };
  }, [dirty]);

  const setDraft = (d: ReviewDocument, next: Draft) => {
    setDrafts((all) => ({ ...all, [d.key]: next }));
    window.clearTimeout(timers.current.get(d.key));
    timers.current.set(
      d.key,
      window.setTimeout(() => {
        client.saveDraft(d.key, d.round, next).catch(() => undefined);
      }, AUTOSAVE_MS),
    );
  };

  const settle = async (d: ReviewDocument, action: () => Promise<unknown>) => {
    try {
      await action();
      setStale((all) => without(all, d.key));
      setDrafts((all) => without(all, d.key));
    } catch (err) {
      if (err instanceof StaleError) {
        setStale((all) => ({ ...all, [d.key]: err.detail }));
        return;
      }
      throw err;
    } finally {
      await queryClient.invalidateQueries({ queryKey: SESSION_KEY });
    }
  };

  if (session.isPending) {
    return (
      <p className="p-8 font-mono text-fg-tertiary">Loading the review…</p>
    );
  }
  if (session.isError) {
    return (
      <p role="alert" className="p-8 font-mono text-st-rejected">
        The review server refused this page: {session.error.message}. Open the
        link docz-review printed.
      </p>
    );
  }
  if (closed !== null) {
    return (
      <p className="p-8 font-mono text-fg-secondary">
        The review session ended ({closed}). You can close this tab.
      </p>
    );
  }

  const allDone = documents.every((d) => d.state !== "open");
  return (
    <div className="mx-auto max-w-[1280px] p-6">
      <nav
        aria-label="Documents"
        className="mb-6 flex flex-wrap items-center gap-2"
      >
        {documents.map((d) => (
          <ReviewButton
            key={d.key}
            aria-current={d.key === doc?.key ? "page" : undefined}
            variant={d.key === doc?.key ? "selected" : "plain"}
            onClick={() => {
              setActive(d.key);
            }}
          >
            {d.key} · {d.state}
          </ReviewButton>
        ))}
        <ReviewButton
          className="ml-auto"
          onClick={() => {
            if (
              !dirty ||
              window.confirm("Leave without an outcome for the open documents?")
            ) {
              void client.done().then(() => {
                setClosed("done");
              });
            }
          }}
        >
          Done
        </ReviewButton>
      </nav>
      {allDone && (
        <p className="mb-4 font-mono text-[12.5px] text-fg-secondary">
          Every document has an outcome. You can close this tab.
        </p>
      )}
      {doc !== undefined && (
        <DocumentReview
          key={`${doc.key}:${doc.round}`}
          doc={doc}
          draft={draftFor(doc)}
          onDraft={(next) => {
            setDraft(doc, next);
          }}
          newSubmissionId={newSubmissionId}
          stale={stale[doc.key] ?? null}
          onReload={() => {
            void queryClient.invalidateQueries({ queryKey: SESSION_KEY });
          }}
          actions={{
            submit: (body) => settle(doc, () => client.submit(doc.key, body)),
            approve: (key) => settle(doc, () => client.approve(doc.key, key)),
            dismiss: (key) => settle(doc, () => client.dismiss(doc.key, key)),
          }}
        />
      )}
    </div>
  );
}
