import {
  DataTable,
  dataTableColumns,
} from "@donaldgifford/design-system/blocks/data-table";
import { PageHeader } from "@donaldgifford/design-system/blocks/page-header";
import { useMemo } from "react";
import { Link, useParams } from "react-router";

import { useGetRepo, useListDocs } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { StatusPill, TypeBadge } from "@/components/badges";
import { TocList } from "@/components/doc-rail";
import {
  EmptyPanel,
  ErrorPanel,
  NotFoundPanel,
  ProseSkeleton,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { DocSurface } from "@/components/doc-surface";
import { typeColor } from "@/lib/colors";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
import { resolveDocType, typeBlurb } from "@/lib/docTypes";
import { arr } from "@/lib/wire";

import type { DocType, Document } from "@/api/__generated__/docz-api.schemas";

/*
 * Type page synthesized from what docz's generated README holds
 * (DESIGN-0001 "Type pages"): plural label, curated blurb, docz-create
 * hint, and the type's documents. No repo file is fetched — everything
 * derives from listTypes + listDocs; if docz-api ever serves the real
 * type README.md it renders in place of this synthesis.
 *
 * The hint is a document, in the document theme. The table is the
 * design system's DataTable (DESIGN-0028 §7) and sits under it, outside
 * the document surface, whose prose rules would restyle it.
 */

const column = dataTableColumns<Document>();

/**
 * A type's documents, in the order docz-api lists them: the table has
 * no sort of its own, as it had none before.
 */
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
  const columns = useMemo(() => {
    const [owner = "", repo = ""] = repoId.split("/");
    return column.columns([
      column.accessor("doc_id", {
        header: "ID",
        cell: ({ row }) => {
          const prefetch = () => {
            prefetchDoc(owner, repo, docType.name, row.original.doc_id);
          };
          return (
            <Link
              to={`/${repoId}/${docType.name}/${row.original.doc_id}`}
              onMouseEnter={prefetch}
              onFocus={prefetch}
              className="font-mono text-xs whitespace-nowrap text-primary hover:underline"
            >
              {row.original.doc_id}
            </Link>
          );
        },
      }),
      // A title and a file name wrap; the table never scrolls sideways
      // for one long row.
      column.accessor("title", {
        header: "Title",
        meta: { className: "whitespace-normal" },
      }),
      column.accessor("status", {
        header: "Status",
        cell: ({ row }) =>
          row.original.status === "" ? (
            <span className="text-muted-foreground">—</span>
          ) : (
            <StatusPill status={row.original.status} />
          ),
      }),
      column.accessor("created", {
        header: "Date",
        cell: ({ row }) => (
          <span className="font-mono text-xs whitespace-nowrap text-muted-foreground">
            {row.original.created === "" ? "—" : row.original.created}
          </span>
        ),
      }),
      column.accessor("path", {
        header: "Link",
        meta: { hideBelow: "md", className: "whitespace-normal" },
        cell: ({ row }) => (
          <span className="font-mono text-xs break-all text-muted-foreground">
            {row.original.path.split("/").at(-1) ?? row.original.path}
          </span>
        ),
      }),
    ]);
  }, [repoId, docType.name, prefetchDoc]);

  return (
    <DataTable
      label={`All ${docType.plural_label}`}
      columns={columns}
      data={docs}
      getRowId={(doc) => doc.doc_id}
      sortable={false}
    />
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
        error={error}
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
      crumbs={[
        { label: repoId, to: `/${repoId}` },
        { label: docType?.name ?? type },
      ]}
      rail={<TocList toc={toc} />}
    >
      {docType === undefined || docs === undefined ? (
        <ProseSkeleton data-testid="repo-type-skeleton" />
      ) : (
        <>
          {/* The page's title is chrome, in the shell's look; the body
              below it is still a document, in the document theme. */}
          <PageHeader
            className="px-0 pt-0"
            title={docType.plural_label}
            meta={<TypeBadge type={docType.name} />}
            description={typeBlurb(docType)}
          />
          <DocSurface accent={typeColor(docType.name)}>
            <article className="doc-prose">
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
            </article>
          </DocSurface>

          <section aria-labelledby="all" className="mt-8">
            <h2 id="all" className="mb-3 text-lg font-semibold">
              All {docType.plural_label}
            </h2>
            {docs.length === 0 ? (
              <EmptyPanel
                title={`No ${docType.name} documents yet`}
                description={
                  <>
                    <code className="font-mono">
                      docz create {docType.name}
                    </code>{" "}
                    scaffolds the first one.
                  </>
                }
              />
            ) : (
              <DocsTable repoId={repoId} docType={docType} docs={docs} />
            )}
          </section>
        </>
      )}
    </RepoFrame>
  );
}
