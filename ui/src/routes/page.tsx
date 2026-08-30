import { useEffect, useMemo } from "react";
import { Navigate, useParams } from "react-router";

import { useGetRepo, useGetRepoPage } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { TocList } from "@/components/doc-rail";
import {
  ErrorPanel,
  NotFoundPanel,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { useRepoDocIndex } from "@/hooks/useRepoDocIndex";
import { apiConfig } from "@/lib/apiConfig";
import { pageSourceKeys, pageSourcePath } from "@/lib/pagePaths";
import { recordRecentDoc } from "@/lib/recentDocs";
import { useRenderedSource } from "@/markdown/useRenderedMarkdown";

/*
 * Published-page reader (DESIGN-0004 Component 4): the api: block's
 * non-docz markdown at /:owner/:repo/pages/<published path>. Renders
 * like the repo home and changelog — the page's own h1 kept (it IS
 * the title), ToC rail, the one sanitizing pipeline. A 404 is the
 * neutral panel (upstream makes invalid paths indistinguishable from
 * misses); a 503 stays the retryable panel (DESIGN-0003). An empty
 * splat redirects home: the landing page IS the repo home.
 */

function PageSkeleton() {
  return (
    <div aria-hidden data-testid="page-skeleton" className="animate-pulse">
      <div className="mb-6 h-8 w-2/5 bg-bg-elevated" />
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
  const params = useParams();
  const { owner, repo } = params;
  const splat = params["*"] ?? "";
  if (owner === undefined || repo === undefined) {
    throw new Error("page route rendered without its path params");
  }
  const repoId = `${owner}/${repo}`;

  const repoQuery = useGetRepo(owner, repo);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;

  // Orval's URL builder interpolates the param RAW — encode the whole
  // published path as one segment (the spelling the spec blesses).
  // Never string-build the API URL by hand.
  const pageQuery = useGetRepoPage(owner, repo, encodeURIComponent(splat), {
    query: { enabled: splat !== "" },
  });
  const page = pageQuery.data?.status === 200 ? pageQuery.data.data : undefined;

  // The page's own source path anchors relative links (base) and is
  // dropped from the target map — a self-link reads oddly.
  const docIndex = useRepoDocIndex(owner, repo);
  const sourceInputs = useMemo(() => {
    if (detail === undefined) {
      return undefined;
    }
    const cfg = apiConfig(detail.config_snapshot);
    const additionalDocs = cfg?.additionalDocs ?? [];
    return {
      base: pageSourcePath(splat, detail.docs_dir, additionalDocs),
      ownKeys: pageSourceKeys(splat, detail.docs_dir, additionalDocs),
    };
  }, [detail, splat]);
  const links = useMemo(() => {
    if (docIndex === undefined || sourceInputs === undefined) {
      return undefined;
    }
    const withoutSelf = new Map(docIndex.byPath);
    for (const key of sourceInputs.ownKeys) {
      withoutSelf.delete(key);
    }
    return {
      xrefs: docIndex.byId,
      paths: withoutSelf,
      base: sourceInputs.base,
    };
  }, [docIndex, sourceInputs]);

  // Successful loads feed the palette's empty-query "recent" group.
  useEffect(() => {
    if (page !== undefined) {
      recordRecentDoc({
        kind: "page",
        repo: page.repo,
        path: page.path,
        title: page.title,
      });
    }
  }, [page]);

  const rendered = useRenderedSource(
    page === undefined
      ? undefined
      : {
          id: `repo-page:${repoId}:${page.path}`,
          hash: page.git_sha,
          raw: page.raw_md,
        },
    links,
  );

  if (splat === "") {
    return <Navigate to={`/${repoId}`} replace />;
  }

  if (
    repoQuery.error instanceof SessionRequiredError ||
    pageQuery.error instanceof SessionRequiredError
  ) {
    return <SessionRequiredRedirect />;
  }
  // The repo is missing/hidden, or the page path isn't published —
  // upstream keeps invalid paths indistinguishable from misses, and so
  // does this panel.
  if (
    repoQuery.error instanceof NotFoundError ||
    pageQuery.error instanceof NotFoundError
  ) {
    return <NotFoundPanel />;
  }

  const failed = [repoQuery, pageQuery].find((query) => query.isError);
  const content = (() => {
    if (failed !== undefined) {
      return (
        <ErrorPanel
          message={
            failed.error instanceof Error
              ? failed.error.message
              : "Request failed"
          }
          onRetry={() => {
            void failed.refetch();
          }}
        />
      );
    }
    if (page === undefined || rendered.data === undefined) {
      return <PageSkeleton />;
    }
    return (
      <>
        <article className="doc-prose">{rendered.data.content}</article>
        <div
          data-testid="page-meta"
          className="mt-10 border-t border-border-hairline pt-3 font-mono text-[11px] text-fg-muted"
        >
          {sourceInputs?.base ?? page.path} · {page.git_sha.slice(0, 7)}
        </div>
      </>
    );
  })();

  return (
    <RepoFrame
      owner={owner}
      name={repo}
      crumbs={[
        { label: repoId, to: `/${repoId}` },
        { label: "pages" },
        ...splat.split("/").map((segment) => ({ label: segment })),
      ]}
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
