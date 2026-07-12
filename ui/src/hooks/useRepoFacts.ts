import { useSearchDocs } from "@/api/__generated__/docz-api";

/*
 * Per-repo doc counts (DESIGN-0001 "Repos and repo pages"):
 * RepoSummary carries no counts, so totals and the per-type split come
 * from a repo-filtered searchDocs facet query. limit 0 — facets and
 * estimated_total_hits cover the whole filtered set. Cached per repo by
 * the query key; shared by the repos grid and the repo nav so their
 * numbers always agree.
 */

export interface RepoFacts {
  /** Total indexed docs in the repo. */
  total: number;
  /** Canonical type name -> doc count. */
  typeCounts: Record<string, number>;
}

export function useRepoFacts(repo: string): {
  facts: RepoFacts | undefined;
  isError: boolean;
} {
  const query = useSearchDocs(
    { repo, limit: 0 },
    { query: { staleTime: 5 * 60_000 } },
  );
  const result = query.data?.status === 200 ? query.data.data : undefined;
  return {
    facts:
      result === undefined
        ? undefined
        : {
            total: result.estimated_total_hits,
            typeCounts: result.facets.type ?? {},
          },
    isError: query.isError,
  };
}
