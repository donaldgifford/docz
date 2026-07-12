import { Link, useParams } from "react-router";

import { useGetRepo, useListDocs } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { StatusBadge } from "@/components/badges";
import { TocList } from "@/components/doc-rail";
import {
  ErrorPanel,
  NotFoundPanel,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
import { resolveDocType, typeBlurb } from "@/lib/docTypes";
import { arr } from "@/lib/wire";

import type { DocType, Document } from "@/api/__generated__/docz-api.schemas";

/*
 * Type page synthesized in the shape of docz's generated README tables
 * (DESIGN-0001 "Type pages"): plural label, curated blurb, docz-create
 * hint, and the doc table between "docz auto-generated" markers. No
 * repo file is fetched — everything derives from listTypes + listDocs;
 * if docz-api ever serves the real type README.md it renders in place
 * of this synthesis.
 */

function GenMarker({ children }: { children: string }) {
  return (
    <div className="my-6 flex items-center gap-3 font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase before:h-px before:flex-1 before:bg-border-hairline before:content-[''] after:h-px after:flex-1 after:bg-border-hairline after:content-['']">
      {children}
    </div>
  );
}

function DocsTable({
  repoId,
  docType,
  docs,
}: {
  repoId: string;
  docType: DocType;
  docs: readonly Document[];
}) {
  const prefetchDoc = usePrefetchDoc();
  return (
    <table className="w-full border-collapse text-[13px]">
      <thead>
        <tr>
          {["ID", "Title", "Status", "Date", "Link"].map((heading) => (
            <th
              key={heading}
              className="border-b border-border-default px-[0.7rem] py-2 text-left font-mono text-[11px] font-normal tracking-[0.05em] text-fg-muted uppercase"
            >
              {heading}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {docs.map((doc) => {
          const [tableOwner = "", tableRepo = ""] = repoId.split("/");
          const prefetch = () => {
            prefetchDoc(tableOwner, tableRepo, docType.name, doc.doc_id);
          };
          return (
            <tr key={doc.doc_id}>
              <td className="border-b border-border-hairline px-[0.7rem] py-2 align-top whitespace-nowrap">
                <Link
                  to={`/${repoId}/${docType.name}/${doc.doc_id}`}
                  onMouseEnter={prefetch}
                  onFocus={prefetch}
                  className="font-mono text-[12px] text-accent hover:underline"
                >
                  {doc.doc_id}
                </Link>
              </td>
              <td className="border-b border-border-hairline px-[0.7rem] py-2 align-top text-fg-primary">
                {doc.title}
              </td>
              <td className="border-b border-border-hairline px-[0.7rem] py-2 align-top">
                {doc.status === "" ? (
                  <span className="text-fg-muted">—</span>
                ) : (
                  <StatusBadge status={doc.status} />
                )}
              </td>
              <td className="border-b border-border-hairline px-[0.7rem] py-2 align-top font-mono text-[11.5px] text-fg-tertiary">
                {doc.created === "" ? "—" : doc.created}
              </td>
              <td className="border-b border-border-hairline px-[0.7rem] py-2 align-top font-mono text-[11.5px] break-all text-fg-tertiary">
                {doc.path.split("/").at(-1) ?? doc.path}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function TypeSkeleton() {
  return (
    <div aria-hidden data-testid="repo-type-skeleton" className="animate-pulse">
      <div className="mb-6 h-8 w-2/5 bg-bg-elevated" />
      <div className="space-y-3">
        {Array.from({ length: 5 }, (_, i) => (
          <div
            key={i}
            className="h-3 bg-bg-raised"
            style={{ width: `${String(94 - (i % 3) * 10)}%` }}
          />
        ))}
      </div>
    </div>
  );
}

export function Component() {
  const { owner, repo, type } = useParams();
  if (owner === undefined || repo === undefined || type === undefined) {
    throw new Error("repo-type route rendered without its path params");
  }
  const repoId = `${owner}/${repo}`;

  const repoQuery = useGetRepo(owner, repo);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;
  // The URL may carry an alias or id_prefix; resolve to the canonical
  // type for display (the API resolves its own paths the same way).
  const docType =
    detail === undefined ? undefined : resolveDocType(arr(detail.types), type);

  const docsQuery = useListDocs(owner, repo, type);
  const docs =
    docsQuery.data?.status === 200 ? arr(docsQuery.data.data.docs) : undefined;

  if (
    repoQuery.error instanceof SessionRequiredError ||
    docsQuery.error instanceof SessionRequiredError
  ) {
    return <SessionRequiredRedirect />;
  }
  if (
    repoQuery.error instanceof NotFoundError ||
    docsQuery.error instanceof NotFoundError ||
    (detail !== undefined && docType === undefined)
  ) {
    return <NotFoundPanel />;
  }
  if (repoQuery.isError || docsQuery.isError) {
    const error = repoQuery.error ?? docsQuery.error;
    return (
      <ErrorPanel
        message={error instanceof Error ? error.message : "Request failed"}
        onRetry={() => {
          void (repoQuery.isError ? repoQuery.refetch() : docsQuery.refetch());
        }}
      />
    );
  }

  const toc =
    docType === undefined
      ? []
      : [
          {
            depth: 2 as const,
            text: `Creating a new ${docType.name}`,
            id: "creating",
          },
          { depth: 2 as const, text: `All ${docType.plural_label}`, id: "all" },
        ];

  return (
    <RepoFrame
      owner={owner}
      name={repo}
      crumbs={[
        { label: repoId, to: `/${repoId}` },
        { label: docType?.name ?? type },
      ]}
      rail={
        <>
          <div className="mb-3 border-b border-border-hairline pb-2 font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase">
            On this page
          </div>
          <TocList toc={toc} />
        </>
      }
    >
      {docType === undefined || docs === undefined ? (
        <TypeSkeleton />
      ) : (
        <article className="doc-prose">
          <h1 className="mb-4 font-serif text-[clamp(1.7rem,4vw,2.3rem)] leading-[1.15] font-normal tracking-[-0.02em] text-fg-primary">
            {docType.plural_label}
          </h1>
          <p className="mb-7 text-[14px] text-fg-tertiary">
            {typeBlurb(docType)}
          </p>

          <h2 id="creating">Creating a new {docType.name}</h2>
          {/* eslint-disable jsx-a11y/no-noninteractive-tabindex --
              scrollable region must be keyboard-reachable (see MarkdownPre) */}
          <pre
            role="region"
            aria-label="code block"
            tabIndex={0}
            className="overflow-x-auto"
          >
            <code>{`$ docz create ${docType.name} "Your ${docType.name} title"`}</code>
          </pre>
          {/* eslint-enable jsx-a11y/no-noninteractive-tabindex */}

          <GenMarker>begin docz auto-generated</GenMarker>
          <h2 id="all">All {docType.plural_label}</h2>
          {docs.length === 0 ? (
            <p className="font-mono text-[13px] text-fg-muted">
              No {docType.name} documents yet —{" "}
              <code>docz create {docType.name}</code> scaffolds the first one.
            </p>
          ) : (
            <DocsTable repoId={repoId} docType={docType} docs={docs} />
          )}
          <GenMarker>end docz auto-generated</GenMarker>
        </article>
      )}
    </RepoFrame>
  );
}
