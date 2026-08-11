import { useParams } from "react-router";

import { useGetRepo, useGetRepoChangelog } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import { TocList } from "@/components/doc-rail";
import {
  ErrorPanel,
  NotFoundPanel,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { RepoFrame } from "@/components/repo-frame";
import { useRepoDocIndex } from "@/hooks/useRepoDocIndex";
import { changelogConfig } from "@/lib/changelogConfig";
import { useRenderedSource } from "@/markdown/useRenderedMarkdown";

/*
 * Repo changelog page (DESIGN-0002 Component 3, from INV-0005): the
 * `.docz.yaml`-opt-in changelog served by getRepoChangelog since spec
 * 1.2.0, rendered exactly like the repo home renders index.md — the
 * file's own h1 kept (it IS the page title), ToC rail as a version
 * jump list. A changelog 404 is never an error here: it means the
 * repo doesn't serve one (or the configured file is absent at HEAD) —
 * quiet panel, not red.
 */

const TITLE_CLASS =
  "mb-4 font-serif text-[clamp(1.7rem,4vw,2.3rem)] leading-[1.15] font-normal tracking-[-0.02em] text-fg-primary";

function ChangelogSkeleton() {
  return (
    <div
      aria-hidden
      data-testid="repo-changelog-skeleton"
      className="animate-pulse"
    >
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

/**
 * The repo serves no changelog: either the `changelog:` block is not
 * enabled, or it is and the configured file is absent at HEAD — the
 * latter is worth saying precisely, as honest feedback to the repo
 * owner who opted in.
 */
function QuietPanel({ file }: { file: string | undefined }) {
  return (
    <div>
      <h1 className={TITLE_CLASS}>Changelog</h1>
      <p className="text-[14px] text-fg-tertiary">
        {file === undefined ? (
          <>
            This repository doesn&rsquo;t serve a changelog — no enabled{" "}
            <code className="font-mono text-[0.85em] text-accent">
              changelog:
            </code>{" "}
            block in its docz.yaml.
          </>
        ) : (
          <>
            The configured changelog{" "}
            <code className="font-mono text-[0.85em] text-accent">{file}</code>{" "}
            is absent at HEAD.
          </>
        )}
      </p>
    </div>
  );
}

function EmptyPanel({ file }: { file: string }) {
  return (
    <div>
      <h1 className={TITLE_CLASS}>Changelog</h1>
      <p className="text-[14px] text-fg-tertiary">
        <code className="font-mono text-[0.85em] text-accent">{file}</code> is
        empty at HEAD.
      </p>
    </div>
  );
}

export function Component() {
  const { owner, repo } = useParams();
  if (owner === undefined || repo === undefined) {
    throw new Error("repo-changelog route rendered without its path params");
  }
  const repoId = `${owner}/${repo}`;

  const repoQuery = useGetRepo(owner, repo);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;
  const cfg = changelogConfig(detail?.config_snapshot);

  const changelogQuery = useGetRepoChangelog(owner, repo);
  const changelog =
    changelogQuery.data?.status === 200 ? changelogQuery.data.data : undefined;
  const noChangelog = changelogQuery.error instanceof NotFoundError;

  // Doc-id tokens in the changelog body link to their readers.
  const docIndex = useRepoDocIndex(owner, repo);
  // Cached per (repo, changelog_sha) — the reader's (doc_id,
  // content_hash) memoization, with the blob SHA as the hash.
  const rendered = useRenderedSource(
    changelog === undefined || changelog.changelog_md === ""
      ? undefined
      : {
          id: `repo-changelog:${repoId}`,
          hash: changelog.changelog_sha,
          raw: changelog.changelog_md,
        },
    docIndex === undefined ? undefined : { xrefs: docIndex },
  );

  if (
    repoQuery.error instanceof SessionRequiredError ||
    changelogQuery.error instanceof SessionRequiredError
  ) {
    return <SessionRequiredRedirect />;
  }
  // The repo itself is missing (or hidden) — not just its changelog.
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
    if (noChangelog) {
      // Wait for the repo detail so the panel can say WHY (config
      // disabled vs file absent) instead of guessing.
      return detail === undefined ? (
        <ChangelogSkeleton />
      ) : (
        <QuietPanel file={cfg?.file} />
      );
    }
    if (changelogQuery.isError) {
      return (
        <ErrorPanel
          message={
            changelogQuery.error instanceof Error
              ? changelogQuery.error.message
              : "Request failed"
          }
          onRetry={() => {
            void changelogQuery.refetch();
          }}
        />
      );
    }
    if (changelog?.changelog_md === "") {
      return <EmptyPanel file={cfg?.file ?? "CHANGELOG.md"} />;
    }
    if (rendered.data === undefined) {
      return <ChangelogSkeleton />;
    }
    return <article className="doc-prose">{rendered.data.content}</article>;
  })();

  return (
    <RepoFrame
      owner={owner}
      name={repo}
      crumbs={[{ label: repoId, to: `/${repoId}` }, { label: "changelog" }]}
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
