import { InlineLoading } from "@donaldgifford/design-system/blocks/inline-state";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@donaldgifford/design-system/components/tabs";
import { ModeToggle } from "@donaldgifford/design-system/shell/mode-toggle";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { refreshMermaidTheme } from "@/lib/diagram-theme";
import {
  DEFAULT_THEME,
  newSubmissionId,
  StaleError,
  type ReviewClient,
  type ReviewDocument,
} from "@/review/api";
import { useAppearance } from "@/lib/appearance";
import { ConfirmDialog, ReviewButton, StateBadge } from "@/review/controls";
import { DocumentReview } from "@/review/document-review";
import { emptyDraft, type Draft } from "@/review/draft";

/*
 * The review page (DESIGN-0024 section 5): a tab per document in the
 * round, one DocumentReview at a time, and Done. Drafts live here,
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
  const appearance = useAppearance(refreshMermaidTheme);
  const theme = session.data?.theme ?? DEFAULT_THEME;
  const [active, setActive] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [stale, setStale] = useState<Record<string, string>>({});
  const [closed, setClosed] = useState<string | null>(null);
  const [leaving, setLeaving] = useState(false);
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

  const done = () => {
    void client.done().then(() => {
      setClosed("done");
    });
  };

  if (session.isPending) {
    return <InlineLoading className="p-8">Loading the review…</InlineLoading>;
  }
  if (session.isError) {
    return (
      <p role="alert" className="p-8 text-base text-destructive">
        The review server refused this page: {session.error.message}. Open the
        link docz-review printed.
      </p>
    );
  }
  if (closed !== null) {
    return (
      <p className="p-8 text-base text-muted-foreground">
        The review session ended ({closed}). You can close this tab.
      </p>
    );
  }

  const allDone = documents.every((d) => d.state !== "open");
  return (
    <>
      <header className="sticky top-0 z-40 flex h-11 items-center gap-4 border-b bg-background px-4">
        <span className="flex items-center gap-2">
          <span
            aria-hidden
            className="grid size-5 place-items-center rounded-sm bg-primary font-mono text-xs font-bold text-primary-foreground"
          >
            D
          </span>
          <span className="text-base font-semibold text-foreground">docz</span>
          <span className="text-base text-muted-foreground">· review</span>
        </span>
        <div className="ml-auto flex items-center gap-2">
          <ModeToggle mode={appearance.mode} onToggle={appearance.toggleMode} />
          <ReviewButton
            onClick={() => {
              if (dirty) {
                setLeaving(true);
              } else {
                done();
              }
            }}
          >
            Done
          </ReviewButton>
        </div>
      </header>
      <ConfirmDialog
        open={leaving}
        onOpenChange={setLeaving}
        title="Leave without an outcome for the open documents?"
        description="Done ends the review session. A draft that was not submitted is not sent."
        confirm="Leave"
        onConfirm={done}
      />
      <Tabs
        className="mx-auto max-w-[1280px] gap-6 p-6"
        value={doc?.key ?? null}
        onValueChange={(key) => {
          setActive(String(key));
        }}
        render={<main />}
      >
        <TabsList
          variant="line"
          aria-label="Documents"
          className="max-w-full justify-start overflow-x-auto"
        >
          {documents.map((d) => (
            <TabsTrigger key={d.key} value={d.key} data-document={d.key}>
              {d.key}
              <StateBadge state={d.state} />
            </TabsTrigger>
          ))}
        </TabsList>
        {allDone && (
          <p className="text-xs text-muted-foreground">
            Every document has an outcome. You can close this tab.
          </p>
        )}
        {doc !== undefined && (
          <TabsContent value={doc.key}>
            <DocumentReview
              key={`${doc.key}:${doc.round}`}
              doc={doc}
              draft={draftFor(doc)}
              onDraft={(next) => {
                setDraft(doc, next);
              }}
              newSubmissionId={newSubmissionId}
              stale={stale[doc.key] ?? null}
              theme={theme}
              onReload={() => {
                void queryClient.invalidateQueries({ queryKey: SESSION_KEY });
              }}
              actions={{
                submit: (body) =>
                  settle(doc, () => client.submit(doc.key, body)),
                approve: (key) =>
                  settle(doc, () => client.approve(doc.key, key)),
                dismiss: (key) =>
                  settle(doc, () => client.dismiss(doc.key, key)),
              }}
            />
          </TabsContent>
        )}
      </Tabs>
    </>
  );
}
