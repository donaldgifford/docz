import { Link, useParams } from "react-router";

import { useGetRepo, useGetRepoIndex } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { TocList } from "@/components/doc-rail";
import {
  ErrorPanel,
  NotFoundPanel,
  SessionRequiredPanel,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { useRepoFacts } from "@/hooks/useRepoFacts";
import { useRenderedSource } from "@/markdown/useRenderedMarkdown";

import type { RepoDetail } from "@/api/__generated__/docz-api.schemas";

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
      <h1 className="mb-4 font-serif text-[clamp(1.7rem,4vw,2.3rem)] leading-[1.15] font-normal tracking-[-0.02em] text-fg-primary">
        {detail.repo.split("/").at(-1) ?? detail.repo}
      </h1>
      <p className="mb-8 text-[14px] text-fg-tertiary">
        No <code className="font-mono text-[0.85em] text-accent">index.md</code>{" "}
        configured in docz.yaml.
      </p>

      {detail.types.map((docType) => {
        const count = facts?.typeCounts[docType.name] ?? 0;
        return (
          <section
            key={docType.name}
            className="mb-6 border-t border-border-hairline pt-5"
          >
            <h2 className="mb-1 text-[17px] font-semibold text-fg-primary">
              {docType.plural_label}{" "}
              <span className="font-mono text-[12px] font-normal text-fg-muted">
                {count}
              </span>
            </h2>
            <Link
              to={`/${detail.repo}/${docType.name}`}
              className="font-mono text-[12.5px] text-accent hover:underline"
            >
              browse {docType.dir}/ →
            </Link>
          </section>
        );
      })}
    </div>
  );
}

function HomeSkeleton() {
  return (
    <div aria-hidden data-testid="repo-home-skeleton" className="animate-pulse">
      <div className="mb-6 h-8 w-3/5 bg-bg-elevated" />
      <div className="space-y-3">
        {Array.from({ length: 6 }, (_, i) => (
          <div
            key={i}
            className="h-3 bg-bg-raised"
            style={{ width: `${String(96 - (i % 3) * 12)}%` }}
          />
        ))}
      </div>
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

  const rendered = useRenderedSource(
    index === undefined
      ? undefined
      : {
          id: `repo-index:${repoId}`,
          hash: index.index_sha,
          raw: index.index_md,
        },
  );

  if (
    repoQuery.error instanceof SessionRequiredError ||
    indexQuery.error instanceof SessionRequiredError
  ) {
    return <SessionRequiredPanel />;
  }
  // The repo itself is missing (or hidden) — not just its index.md.
  if (repoQuery.error instanceof NotFoundError) {
    return <NotFoundPanel />;
  }
  if (repoQuery.isError) {
    return (
      <ErrorPanel
        message={
          repoQuery.error instanceof Error
            ? repoQuery.error.message
            : "Request failed"
        }
        onRetry={() => {
          void repoQuery.refetch();
        }}
      />
    );
  }

  const content = (() => {
    if (noIndex) {
      return detail === undefined ? (
        <HomeSkeleton />
      ) : (
        <GeneratedHome detail={detail} />
      );
    }
    if (indexQuery.isError) {
      return (
        <ErrorPanel
          message={
            indexQuery.error instanceof Error
              ? indexQuery.error.message
              : "Request failed"
          }
          onRetry={() => {
            void indexQuery.refetch();
          }}
        />
      );
    }
    if (rendered.data === undefined) {
      return <HomeSkeleton />;
    }
    return <article className="doc-prose">{rendered.data.content}</article>;
  })();

  return (
    <RepoFrame
      owner={owner}
      name={repo}
      crumbs={[{ label: repoId }]}
      rail={
        <>
          <div className="mb-3 border-b border-border-hairline pb-2 font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase">
            On this page
          </div>
          <TocList toc={rendered.data?.toc ?? []} />
        </>
      }
    >
      {content}
    </RepoFrame>
  );
}
