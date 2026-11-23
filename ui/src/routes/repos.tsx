import { PageHeader } from "@donaldgifford/design-system/blocks/page-header";
import { Badge } from "@donaldgifford/design-system/components/badge";
import {
  Card,
  CardContent,
} from "@donaldgifford/design-system/components/card";
import { Link } from "react-router";

import { useListRepos } from "@/api/__generated__/docz-api";
import { SessionRequiredError } from "@/api/fetcher";
import {
  EmptyPanel,
  ErrorPanel,
  SessionRequiredRedirect,
  Skeleton,
} from "@/components/query-states";
import { useRepoFacts } from "@/hooks/useRepoFacts";
import { arr } from "@/lib/wire";

import type { RepoSummary } from "@/api/__generated__/docz-api.schemas";

/*
 * /repos grid per the mockup's repo cards. RepoSummary carries no
 * counts — each card fills them in from useRepoFacts (shared with the
 * repo nav, so numbers agree everywhere). No last-updated exists in the
 * contract either; the card shows the last-synced SHA instead.
 */

/** The page's column, shared by the grid and its skeleton. */
const GRID = "grid grid-cols-1 gap-4 md:grid-cols-2";

function RepoCard({ repo }: { repo: RepoSummary }) {
  const { facts } = useRepoFacts(repo.repo);
  const typeCounts = Object.entries(facts?.typeCounts ?? {}).sort(([a], [b]) =>
    a.localeCompare(b),
  );

  return (
    // The whole card is the link: one target, one focus ring.
    <Card
      render={<Link to={`/${repo.repo}`} />}
      className="transition-colors hover:border-border-strong focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <CardContent className="flex items-center gap-2.5">
        <span className="truncate font-semibold">{repo.repo}</span>
        <Badge variant="outline" className="ml-auto">
          {repo.default_branch}
        </Badge>
      </CardContent>

      <CardContent className="flex min-h-4 flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        {typeCounts.map(([type, count]) => (
          <span key={type}>
            <b className="font-medium text-foreground">{count}</b> {type}
          </span>
        ))}
      </CardContent>

      <CardContent className="flex flex-wrap gap-4 font-mono text-xs text-muted-foreground">
        <span>{repo.docs_dir}/</span>
        <span>
          docs:{" "}
          <b className="font-medium text-foreground">{facts?.total ?? "—"}</b>
        </span>
        <span className="ml-auto">
          {repo.last_synced_sha === ""
            ? "never synced"
            : `sync ${repo.last_synced_sha.slice(0, 7)}`}
        </span>
      </CardContent>
    </Card>
  );
}

function CardSkeletons() {
  return (
    <div aria-hidden data-testid="repos-skeleton" className={GRID}>
      {Array.from({ length: 4 }, (_, i) => (
        <Skeleton key={i} className="h-24" />
      ))}
    </div>
  );
}

export function Component() {
  const reposQuery = useListRepos();
  const repos =
    reposQuery.data?.status === 200
      ? arr(reposQuery.data.data.repos)
      : undefined;

  if (reposQuery.error instanceof SessionRequiredError) {
    return <SessionRequiredRedirect />;
  }

  return (
    <main className="mx-auto max-w-[940px] px-5">
      <PageHeader
        className="px-0"
        title="Repositories"
        description={
          <>
            Every repo registered with docz-api. Each renders its own homepage
            from the <code className="font-mono">index.md</code> its{" "}
            <code className="font-mono">docz.yaml</code> points at, plus a
            section per configured doc type.
          </>
        }
      />

      {reposQuery.isError ? (
        <ErrorPanel
          error={reposQuery.error}
          onRetry={() => {
            void reposQuery.refetch();
          }}
        />
      ) : repos === undefined ? (
        <CardSkeletons />
      ) : repos.length === 0 ? (
        <EmptyPanel
          title="No repositories yet"
          description="Onboard a repo with the docz GitHub App to see it here."
        />
      ) : (
        <div className={`mb-16 ${GRID}`}>
          {repos.map((repo) => (
            <RepoCard key={repo.repo} repo={repo} />
          ))}
        </div>
      )}
    </main>
  );
}
