import { useState } from "react";
import { Link, useParams } from "react-router";

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
  SessionRequiredPanel,
} from "@/components/query-states";
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
  const fileName = doc.path.split("/").at(-1) ?? doc.path;
  const parts = docIdParts(doc.doc_id);
  const updated = doc.updated_at === "" ? "—" : doc.updated_at.slice(0, 10);

  return (
    <header className="mb-9 border-b border-border-hairline pb-8">
      {/* File-path breadcrumb: repos / owner/name / type / filename */}
      <nav
        aria-label="Breadcrumb"
        className="mb-6 flex flex-wrap gap-2 font-mono text-[12px] text-fg-muted"
      >
        <Link to="/repos" className="text-fg-tertiary hover:text-fg-primary">
          repos
        </Link>
        <span>/</span>
        <Link
          to={`/${doc.repo}`}
          className="text-fg-tertiary hover:text-fg-primary"
        >
          {doc.repo}
        </Link>
        <span>/</span>
        <Link
          to={`/${doc.repo}/${doc.type}`}
          className="text-fg-tertiary hover:text-fg-primary"
        >
          {doc.type}
        </Link>
        <span>/</span>
        <span className="text-accent [overflow-wrap:anywhere]">{fileName}</span>
      </nav>

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
  const rendered = useRenderedMarkdown(doc);
  const [format, setFormat] = useState<DocFormat>("html");

  if (docQuery.error instanceof SessionRequiredError) {
    return <SessionRequiredPanel />;
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
  if (doc === undefined || rendered.data === undefined) {
    return <ArticleSkeleton />;
  }

  const { content, toc } = rendered.data;

  return (
    <div className="mx-auto grid max-w-6xl grid-cols-1 gap-10 px-6 py-10 lg:grid-cols-[minmax(0,1fr)_230px]">
      <main className="min-w-0">
        <DocHeader doc={doc} />

        {/* Narrow viewports: ToC as a disclosure above the article. */}
        <details className="mb-6 border border-border-hairline px-4 py-3 lg:hidden">
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
          <pre className="overflow-x-auto border border-border-default bg-code-bg p-5 font-mono text-[12.5px] leading-[1.55] whitespace-pre-wrap text-fg-secondary">
            {doc.raw_md ?? ""}
          </pre>
        )}
      </main>

      <aside className="hidden lg:block">
        <div className="sticky top-[76px]">
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
        </div>
      </aside>
    </div>
  );
}
