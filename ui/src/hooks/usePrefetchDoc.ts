import { useQueryClient } from "@tanstack/react-query";

import { getGetDocQueryOptions } from "@/api/__generated__/docz-api";

/*
 * Hover/focus prefetch for doc links (IMPL-0001 Phase 4 performance):
 * by the time the click lands, the getDoc response is usually cached
 * and the reader paints without a skeleton. staleTime only applies to
 * the prefetch — the mounted useGetDoc revalidates in the background
 * per its own defaults, so a stale hover can't pin old content.
 */

const PREFETCH_STALE_MS = 30_000;

export function usePrefetchDoc(): (
  owner: string,
  name: string,
  type: string,
  docId: string,
) => void {
  const queryClient = useQueryClient();
  return (owner, name, type, docId) => {
    void queryClient.prefetchQuery(
      getGetDocQueryOptions(owner, name, type, docId, {
        query: { staleTime: PREFETCH_STALE_MS },
      }),
    );
  };
}
