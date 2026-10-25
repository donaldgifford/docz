import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";

import { StatusPill } from "@/components/badges";
import { renderMarkdown } from "@/markdown/processor";
import { resolveAnchor } from "@/review/anchor";
import {
  AnnotationList,
  DocumentComment,
  SelectionPopover,
  useAnnotationPaint,
  useSelectionAnchor,
} from "@/review/annotations";
import { ReviewButton } from "@/review/controls";
import { compareRounds, markChangedRegions, RoundDiff } from "@/review/diff";
import {
  canApprove,
  hasActionable,
  toRecordParts,
  type Draft,
} from "@/review/draft";
import { Findings } from "@/review/findings";
import { QuestionCards } from "@/review/questions";
import { StatusSelect } from "@/review/status";

import type { ReviewDocument, Submission, SubmissionKey } from "@/review/api";

/*
 * One document's review: findings above the text, the diff since the
 * previous round, the document rendered in review mode with its regions
 * wrapped, and a sidebar with the question cards, the annotations, the
 * status, and the per-document actions (DESIGN-0024 section 5).
 */

export interface DocumentActions {
  submit: (body: Submission) => Promise<void>;
  approve: (key: SubmissionKey) => Promise<void>;
  dismiss: (key: SubmissionKey) => Promise<void>;
}

function FrontmatterRows({ doc }: { doc: ReviewDocument }) {
  const fm = doc.inspect.frontmatter;
  if (fm === null) {
    return (
      <p className="font-mono text-[12.5px] text-st-rejected">
        This document has no frontmatter.
      </p>
    );
  }
  const rows: [string, string][] = [
    ["id", fm.id],
    ["author", fm.author],
    ["created", fm.created],
    ["path", doc.inspect.path],
  ];
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 font-mono text-[12.5px]">
      {rows.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="text-fg-muted uppercase">{k}</dt>
          <dd className="text-fg-primary">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function DocumentReview({
  doc,
  draft,
  onDraft,
  actions,
  newSubmissionId,
  stale,
  onReload,
}: {
  doc: ReviewDocument;
  draft: Draft;
  onDraft: (next: Draft) => void;
  actions: DocumentActions;
  newSubmissionId: () => string;
  /** Set when the last submission found the file changed on disk. */
  stale: string | null;
  onReload: () => void;
}) {
  const { inspect } = doc;
  const current = inspect.frontmatter?.status ?? "";
  const open = doc.state === "open";
  const [busy, setBusy] = useState(false);
  const docRef = useRef<HTMLDivElement>(null);

  const rendered = useQuery({
    queryKey: ["review-render", inspect.content_sha256, inspect.path],
    queryFn: () =>
      renderMarkdown(inspect.content, {
        review: true,
        regions: inspect.regions,
      }),
    staleTime: Infinity,
  });
  const comparison = useMemo(
    () =>
      doc.previous === null
        ? null
        : compareRounds(doc.previous, inspect.content),
    [doc.previous, inspect.content],
  );

  const [pending, clearPending] = useSelectionAnchor(docRef, inspect.headings);
  useAnnotationPaint(docRef, draft.annotations, rendered.data);
  useEffect(() => {
    if (docRef.current !== null && comparison !== null) {
      markChangedRegions(docRef.current, comparison.changed);
    }
  }, [comparison, rendered.data]);

  const key = (): SubmissionKey => ({
    round: doc.round,
    content_sha256: inspect.content_sha256,
    submission_id: newSubmissionId(),
  });
  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    try {
      await action();
    } finally {
      setBusy(false);
    }
  };
  const submit = () =>
    run(() =>
      actions.submit({
        ...key(),
        ...toRecordParts(draft, current, (a) =>
          resolveAnchor(a, inspect.content, inspect.regions),
        ),
      }),
    );
  const actionable = hasActionable(draft, current);

  return (
    <div className="grid grid-cols-[minmax(0,1fr)_340px] gap-8">
      <div className="min-w-0">
        <header className="mb-4">
          <p className="font-mono text-[12px] tracking-[0.1em] text-fg-muted uppercase">
            {inspect.id} · round {doc.round_number} · {doc.state}
          </p>
          <h1 className="mt-1 font-serif text-[28px] text-fg-primary">
            {inspect.frontmatter?.title ?? inspect.path}
          </h1>
          <div className="mt-2 flex items-center gap-3">
            {current !== "" && <StatusPill status={current} />}
          </div>
          <div className="mt-3">
            <FrontmatterRows doc={doc} />
          </div>
        </header>

        {stale !== null && (
          <div
            role="alert"
            className="mb-4 border border-st-draft bg-bg-raised p-3 font-mono text-[12.5px] text-st-draft"
          >
            The document changed on disk since this round opened ({stale}). Your
            draft is kept; reload to review the new text before submitting.{" "}
            <ReviewButton onClick={onReload}>Reload</ReviewButton>
          </div>
        )}

        <section aria-label="Validation" className="mb-4">
          <Findings
            findings={doc.findings}
            actions={draft.findings}
            onChange={(findings) => {
              onDraft({ ...draft, findings });
            }}
          />
        </section>

        {comparison !== null && (
          <div className="mb-4">
            <RoundDiff comparison={comparison} />
          </div>
        )}

        <div className="mb-4">
          <DocumentComment
            onAdd={(a) => {
              onDraft({ ...draft, annotations: [...draft.annotations, a] });
            }}
          />
        </div>

        <article ref={docRef} className="doc-prose" aria-label="Document">
          {rendered.data?.content}
        </article>
        {pending !== null && open && (
          <SelectionPopover
            pending={pending}
            onCancel={clearPending}
            onAdd={(a) => {
              onDraft({ ...draft, annotations: [...draft.annotations, a] });
              clearPending();
              document.getSelection()?.removeAllRanges();
            }}
          />
        )}
      </div>

      <aside aria-label="Review" className="flex flex-col gap-4">
        <QuestionCards
          questions={inspect.open_questions}
          answers={draft.questions}
          onChange={(questions) => {
            onDraft({ ...draft, questions });
          }}
        />
        <AnnotationList
          annotations={draft.annotations}
          onEdit={(id, patch) => {
            onDraft({
              ...draft,
              annotations: draft.annotations.map((a) =>
                a.id === id ? { ...a, ...patch } : a,
              ),
            });
          }}
          onRemove={(id) => {
            onDraft({
              ...draft,
              annotations: draft.annotations.filter((a) => a.id !== id),
            });
          }}
        />
        <StatusSelect
          statuses={inspect.statuses}
          current={current}
          value={draft.status}
          onChange={(status) => {
            onDraft({ ...draft, status });
          }}
        />
        {open ? (
          <div className="flex flex-wrap gap-2">
            <ReviewButton
              variant="primary"
              disabled={busy}
              onClick={() => void submit()}
            >
              Submit
            </ReviewButton>
            <ReviewButton
              disabled={busy || !canApprove(draft, current)}
              title={
                actionable
                  ? "The draft holds changes; submit them or discard the draft"
                  : undefined
              }
              onClick={() => void run(() => actions.approve(key()))}
            >
              Approve
            </ReviewButton>
            {actionable && (
              <ReviewButton
                disabled={busy}
                onClick={() => {
                  if (window.confirm("Discard this draft and approve?")) {
                    void run(() => actions.approve(key()));
                  }
                }}
              >
                Discard draft and approve
              </ReviewButton>
            )}
            <ReviewButton
              disabled={busy}
              onClick={() => void run(() => actions.dismiss(key()))}
            >
              Dismiss
            </ReviewButton>
          </div>
        ) : (
          <p className="font-mono text-[12.5px] text-fg-tertiary">
            This round is closed: {doc.state}.
          </p>
        )}
      </aside>
    </div>
  );
}
