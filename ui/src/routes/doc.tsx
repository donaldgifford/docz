import { useMemo, useState } from "react";
import { useParams } from "react-router";

import { useGetDoc } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { StatusPill } from "@/components/badges";
import {
  DocRailInfo,
  LifecycleRail,
  TocList,
  type DocFormat,
} from "@/components/doc-rail";
import {
  ErrorPanel,
  NotFoundPanel,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { useRepoDocIndex } from "@/hooks/useRepoDocIndex";
import { useRenderedMarkdown } from "@/markdown/useRenderedMarkdown";

import type { Document } from "@/api/__generated__/docz-api.schemas";

function ArticleSkeleton() {
  return (
    <div
      aria-hidden
      data-testid="doc-skeleton"
      className="mx-auto max-w-3xl animate-pulse px-6 py-10"
    >
      <div className="h-3 w-40 bg-bg-elevated" />
      <div className="mt-6 h-8 w-4/5 bg-bg-elevated" />
      <div className="mt-3 h-3 w-56 bg-bg-elevated" />
      <div className="mt-10 space-y-3">
        {Array.from({ length: 8 }, (_, i) => (
          <div
            key={i}
            className="h-3 bg-bg-raised"
            style={{ width: `${String(100 - (i % 4) * 9)}%` }}
          />
        ))}
      </div>
    </div>
  );
}

/** "DESIGN-0009" → ["DESIGN", "0009"]; null when there's no separator. */
function docIdParts(docId: string): [string, string] | null {
  const idx = docId.lastIndexOf("-");
  if (idx <= 0 || idx === docId.length - 1) {
    return null;
  }
  return [docId.slice(0, idx).toUpperCase(), docId.slice(idx + 1)];
}

function DocHeader({ doc }: { doc: Document }) {
  const parts = docIdParts(doc.doc_id);
  const updated = doc.updated_at === "" ? "—" : doc.updated_at.slice(0, 10);

  return (
    <header className="mb-9 border-b border-border-hairline pb-8">
      {/* Id line: DESIGN / 0009, with the fading rule from the mockup */}
      <div className="mb-4 flex items-center gap-3 font-mono text-[13px] tracking-[0.04em] text-accent after:h-px after:flex-1 after:bg-gradient-to-r after:from-(--color-accent-border) after:to-transparent after:content-['']">
        <span>{parts ? `${parts[0]} / ${parts[1]}` : doc.doc_id}</span>
      </div>

      <h1 className="font-serif text-[clamp(1.9rem,4.5vw,2.6rem)] leading-[1.14] font-normal tracking-[-0.02em] text-fg-primary">
        {doc.title}
      </h1>

      <div className="mt-4 flex flex-wrap items-center gap-[14px] font-mono text-[12px] text-fg-tertiary">
        {doc.status !== "" && (
          <>
            <StatusPill status={doc.status} />
            <span className="text-fg-muted">·</span>
          </>
        )}
        <span>{doc.author === "" ? "unassigned" : doc.author}</span>
        <span className="text-fg-muted">·</span>
        <span>updated {updated}</span>
      </div>
    </header>
  );
}

export function Component() {
  const { owner, repo, type, docId } = useParams();
  if (
    owner === undefined ||
    repo === undefined ||
    type === undefined ||
    docId === undefined
  ) {
    // The route pattern guarantees these; reaching here is a router bug.
    throw new Error("doc route rendered without its path params");
  }

  const docQuery = useGetDoc(owner, repo, type, docId);
  const doc = docQuery.data?.status === 200 ? docQuery.data.data : undefined;

  // Sibling doc ids resolve to router links inside the body; the doc's
  // own id is excluded (a self-link reads oddly). Until the index
  // loads, the body renders unlinked and is linkified once, cached.
  const docIndex = useRepoDocIndex(owner, repo);
  const xrefs = useMemo(() => {
    if (docIndex === undefined || doc === undefined) {
      return undefined;
    }
    const withoutSelf = new Map(docIndex);
    withoutSelf.delete(doc.doc_id.toUpperCase());
    return withoutSelf;
  }, [docIndex, doc]);

  const rendered = useRenderedMarkdown(doc, xrefs);
  const [format, setFormat] = useState<DocFormat>("html");

  if (docQuery.error instanceof SessionRequiredError) {
    return <SessionRequiredRedirect />;
  }
  if (docQuery.error instanceof NotFoundError) {
    return <NotFoundPanel />;
  }
  if (docQuery.isError || rendered.isError) {
    const error = docQuery.error ?? rendered.error;
    return (
      <ErrorPanel
        message={error instanceof Error ? error.message : "Request failed"}
        onRetry={() => {
          void (docQuery.isError ? docQuery.refetch() : rendered.refetch());
        }}
      />
    );
  }
  const repoId = `${owner}/${repo}`;

  // Reader inside the three-column portal (Phase 3): RepoNav left,
  // article center, ToC + doc info right. While the doc loads, crumbs
  // fall back to the (possibly aliased) route params.
  if (doc === undefined || rendered.data === undefined) {
    return (
      <RepoFrame
        owner={owner}
        name={repo}
        crumbs={[
          { label: repoId, to: `/${repoId}` },
          { label: type, to: `/${repoId}/${type}` },
          { label: docId },
        ]}
      >
        <ArticleSkeleton />
      </RepoFrame>
    );
  }

  const { content, toc } = rendered.data;
  const fileName = doc.path.split("/").at(-1) ?? doc.path;

  return (
    <RepoFrame
      owner={owner}
      name={repo}
      crumbs={[
        { label: repoId, to: `/${repoId}` },
        { label: doc.type, to: `/${repoId}/${doc.type}` },
        { label: fileName },
      ]}
      rail={
        <>
          <section className="mb-8">
            <div className="mb-3 border-b border-border-hairline pb-2 font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase">
              On this page
            </div>
            <TocList toc={toc} />
          </section>
          <DocRailInfo
            doc={doc}
            format={format}
            onFormatChange={setFormat}
            lifecycle={
              <LifecycleRail
                owner={owner}
                name={repo}
                typeName={doc.type}
                currentStatus={doc.status}
              />
            }
          />
        </>
      }
    >
      <DocHeader doc={doc} />

      {/* Narrow viewports: ToC as a disclosure above the article
          (the right rail hides below the frame's 1181px breakpoint). */}
      <details className="mb-6 border border-border-hairline px-4 py-3 min-[1181px]:hidden">
        <summary className="cursor-pointer font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase">
          On this page
        </summary>
        <div className="pt-3">
          <TocList toc={toc} />
        </div>
      </details>

      {format === "html" ? (
        <article className="doc-prose">{content}</article>
      ) : (
        /* eslint-disable jsx-a11y/no-noninteractive-tabindex --
           scrollable region must be keyboard-reachable (see MarkdownPre) */
        <pre
          role="region"
          aria-label="raw markdown"
          tabIndex={0}
          className="overflow-x-auto border border-border-default bg-code-bg p-5 font-mono text-[12.5px] leading-[1.55] whitespace-pre-wrap text-fg-secondary"
        >
          {doc.raw_md ?? ""}
        </pre>
        /* eslint-enable jsx-a11y/no-noninteractive-tabindex */
      )}
    </RepoFrame>
  );
}
