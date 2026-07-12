import { Link } from "react-router";

import { useListRepos } from "@/api/__generated__/docz-api";
import { SessionRequiredError } from "@/api/fetcher";
import { ErrorPanel, SessionRequiredPanel } from "@/components/query-states";
import { useRepoFacts } from "@/hooks/useRepoFacts";

import type { RepoSummary } from "@/api/__generated__/docz-api.schemas";

/*
 * /repos grid per the mockup's repo cards. RepoSummary carries no
 * counts — each card fills them in from useRepoFacts (shared with the
 * repo nav, so numbers agree everywhere). No last-updated exists in the
 * contract either; the card shows the last-synced SHA instead.
 */

function RepoCard({ repo }: { repo: RepoSummary }) {
  const { facts } = useRepoFacts(repo.repo);
  const typeCounts = Object.entries(facts?.typeCounts ?? {}).sort(([a], [b]) =>
    a.localeCompare(b),
  );

  return (
    <Link
      to={`/${repo.repo}`}
      className="block border border-border-default bg-bg-raised px-[1.2rem] py-[1.1rem] transition-colors hover:border-border-strong"
    >
      <div className="mb-3 flex items-center gap-3">
        <span className="font-mono text-[14px] font-semibold text-fg-primary">
          {repo.repo}
        </span>
        <span className="border border-border-default px-[7px] py-px font-mono text-[10.5px] text-fg-muted">
          {repo.default_branch}
        </span>
      </div>

      <div className="mb-3 flex min-h-4 flex-wrap gap-x-4 gap-y-1 font-mono text-[11px] text-fg-tertiary">
        {typeCounts.map(([type, count]) => (
          <span key={type}>
            <b className="font-medium text-fg-secondary">{count}</b> {type}
          </span>
        ))}
      </div>

      <div className="flex flex-wrap gap-4 font-mono text-[11px] text-fg-muted">
        <span className="text-t-framework">{repo.docs_dir}/</span>
        <span>
          docs:{" "}
          <b className="font-medium text-fg-secondary">{facts?.total ?? "—"}</b>
        </span>
        <span className="ml-auto">
          {repo.last_synced_sha === ""
            ? "never synced"
            : `sync ${repo.last_synced_sha.slice(0, 7)}`}
        </span>
      </div>
    </Link>
  );
}

function CardSkeletons() {
  return (
    <div
      aria-hidden
      data-testid="repos-skeleton"
      className="grid animate-pulse grid-cols-1 gap-4 md:grid-cols-2"
    >
      {Array.from({ length: 4 }, (_, i) => (
        <div
          key={i}
          className="h-28 border border-border-hairline bg-bg-raised"
        />
      ))}
    </div>
  );
}

export function Component() {
  const reposQuery = useListRepos();
  const repos =
    reposQuery.data?.status === 200 ? reposQuery.data.data.repos : undefined;

  if (reposQuery.error instanceof SessionRequiredError) {
    return <SessionRequiredPanel />;
  }

  return (
    <main className="mx-auto max-w-[940px] px-5">
      <header className="pt-10 pb-6">
        <div className="font-mono text-[12.5px] tracking-[0.05em] text-accent">
          / docz <span className="text-fg-muted">/</span> repositories
        </div>
        <h1 className="mt-2 mb-1 text-[clamp(1.6rem,4vw,2rem)] font-semibold tracking-[-0.01em] text-fg-primary">
          Repositories
        </h1>
        <p className="text-[14px] text-fg-tertiary">
          Every repo registered with docz-api. Each renders its own homepage
          from the{" "}
          <code className="font-mono text-[0.85em] text-accent">index.md</code>{" "}
          its{" "}
          <code className="font-mono text-[0.85em] text-accent">docz.yaml</code>{" "}
          points at, plus a section per configured doc type.
        </p>
      </header>

      {reposQuery.isError ? (
        <ErrorPanel
          message={
            reposQuery.error instanceof Error
              ? reposQuery.error.message
              : "Request failed"
          }
          onRetry={() => {
            void reposQuery.refetch();
          }}
        />
      ) : repos === undefined ? (
        <CardSkeletons />
      ) : repos.length === 0 ? (
        <div className="mx-auto my-16 w-max max-w-full border border-border-default bg-bg-raised px-8 py-6 text-center">
          <p className="font-mono text-[13px] text-fg-secondary">
            No repositories yet
          </p>
          <p className="mt-2 max-w-96 text-[13px] text-fg-tertiary">
            Onboard a repo with the docz GitHub App to see it here.
          </p>
        </div>
      ) : (
        <div className="mb-16 grid grid-cols-1 gap-4 md:grid-cols-2">
          {repos.map((repo) => (
            <RepoCard key={repo.repo} repo={repo} />
          ))}
        </div>
      )}
    </main>
  );
}
