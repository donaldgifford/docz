import { useSearchDocs } from "@/api/__generated__/docz-api";

import { SearchDocsSource } from "@/api/__generated__/docz-api.schemas";

/*
 * Per-repo doc counts (DESIGN-0001 "Repos and repo pages"):
 * RepoSummary carries no counts, so totals and the per-type split come
 * from a repo-filtered searchDocs facet query. limit 0 — facets and
 * estimated_total_hits cover the whole filtered set. Cached per repo by
 * the query key; shared by the repos grid and the repo nav so their
 * numbers always agree.
 *
 * `source: doc` narrows the query to docz documents (spec 1.5.0). Every
 * consumer here counts docs, and before the filter existed this had to
 * read the doc count back out of the source facet while `type` counted
 * pages too. Asking the server for documents is both simpler and more
 * honest about what the numbers mean.
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
    { repo, limit: 0, source: SearchDocsSource.doc },
    { query: { staleTime: 5 * 60_000 } },
  );
  const result = query.data?.status === 200 ? query.data.data : undefined;
  return {
    facts:
      result === undefined
        ? undefined
        : {
            // The query is already doc-only, so the estimated total IS
            // the doc count. A missing type key means zero (facets omit
            // zero-hit values).
            total: result.estimated_total_hits,
            typeCounts: result.facets.type ?? {},
          },
    isError: query.isError,
  };
}
