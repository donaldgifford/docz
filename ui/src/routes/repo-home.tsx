import { PageHeader } from "@donaldgifford/design-system/blocks/page-header";
import { Link, useParams } from "react-router";

import { useGetRepo, useGetRepoIndex } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { TocList } from "@/components/doc-rail";
import {
  ErrorPanel,
  NotFoundPanel,
  ProseSkeleton,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { DocSurface } from "@/components/doc-surface";
import { useRepoDocIndex } from "@/hooks/useRepoDocIndex";
import { useRepoFacts } from "@/hooks/useRepoFacts";
import { apiConfig } from "@/lib/apiConfig";
import { arr } from "@/lib/wire";
import { useRenderedSource } from "@/markdown/useRenderedMarkdown";

import type { RepoDetail } from "@/api/__generated__/docz-api.schemas";

/**
 * The home body's relative-link base. An enabled api: block may
 * relocate the served landing page (api.landing_page, resolved at
 * ingest) — relative links must anchor to wherever the file actually
 * lives (DESIGN-0004). Empty on wrong shapes, hence `!== ""`, not ??.
 */
function homeBase(detail: RepoDetail): string {
  const landing = apiConfig(detail.config_snapshot)?.landingPage;
  if (landing !== undefined && landing !== "") {
    return landing;
  }
  return detail.docs_dir === "" ? "index.md" : `${detail.docs_dir}/index.md`;
}

/*
 * TechDocs-style repo home (DESIGN-0001 Decision 8 + DESIGN-0003): the
 * repo's docs_dir/index.md — served by getRepoIndex since spec 1.1.0 —
 * rendered through the reader pipeline (leading h1 kept: it IS the page
 * title). A repo without an index.md (404) gets the generated home:
 * the mockup's "No index.md configured" note plus a section per
 * configured doc type.
 */

function GeneratedHome({ detail }: { detail: RepoDetail }) {
  const { facts } = useRepoFacts(detail.repo);
  return (
    <div>
      <PageHeader
        className="px-0 pt-0"
        title={detail.repo.split("/").at(-1) ?? detail.repo}
        description={
          <>
            No <code className="font-mono">index.md</code> configured in
            docz.yaml.
          </>
        }
      />
      {arr(detail.types).map((docType) => {
        const count = facts?.typeCounts[docType.name] ?? 0;
        return (
          <section key={docType.name} className="mb-6 border-t pt-5">
            <h2 className="mb-1 text-lg font-semibold">
              {docType.plural_label}{" "}
              <span className="font-mono text-xs font-normal text-muted-foreground">
                {count}
              </span>
            </h2>
            <Link
              to={`/${detail.repo}/${docType.name}`}
              className="font-mono text-sm text-primary hover:underline"
            >
              browse {docType.dir}/ →
            </Link>
          </section>
        );
      })}
    </div>
  );
}

export function Component() {
  const { owner, repo } = useParams();
  if (owner === undefined || repo === undefined) {
    throw new Error("repo-home route rendered without its path params");
  }
  const repoId = `${owner}/${repo}`;

  const repoQuery = useGetRepo(owner, repo);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;

  const indexQuery = useGetRepoIndex(owner, repo);
  const index =
    indexQuery.data?.status === 200 ? indexQuery.data.data : undefined;
  const noIndex = indexQuery.error instanceof NotFoundError;

  // Doc-id tokens and relative links in the index body resolve to
  // their readers. docIndex is derived from the same getRepo response
  // as detail, so both are present together.
  const docIndex = useRepoDocIndex(owner, repo);
  const rendered = useRenderedSource(
    index === undefined
      ? undefined
      : {
          id: `repo-index:${repoId}`,
          hash: index.index_sha,
          raw: index.index_md,
        },
    docIndex === undefined || detail === undefined
      ? undefined
      : {
          xrefs: docIndex.byId,
          paths: docIndex.byPath,
          base: homeBase(detail),
        },
  );

  if (
    repoQuery.error instanceof SessionRequiredError ||
    indexQuery.error instanceof SessionRequiredError
  ) {
    return <SessionRequiredRedirect />;
  }
  // The repo itself is missing (or hidden) — not just its index.md.
  if (repoQuery.error instanceof NotFoundError) {
    return <NotFoundPanel />;
  }
  if (repoQuery.isError) {
    return (
      <ErrorPanel
        error={repoQuery.error}
        onRetry={() => {
          void repoQuery.refetch();
        }}
      />
    );
  }

  const content = (() => {
    if (noIndex) {
      return detail === undefined ? (
        <ProseSkeleton data-testid="repo-home-skeleton" />
      ) : (
        <GeneratedHome detail={detail} />
      );
    }
    if (indexQuery.isError) {
      return (
        <ErrorPanel
          error={indexQuery.error}
          onRetry={() => {
            void indexQuery.refetch();
          }}
        />
      );
    }
    if (rendered.data === undefined) {
      return <ProseSkeleton data-testid="repo-home-skeleton" />;
    }
    return (
      <DocSurface>
        <article className="doc-prose">{rendered.data.content}</article>
      </DocSurface>
    );
  })();

  return (
    <RepoFrame
      owner={owner}
      name={repo}
      crumbs={[{ label: repoId }]}
      rail={<TocList toc={rendered.data?.toc ?? []} />}
    >
      {content}
    </RepoFrame>
  );
}
