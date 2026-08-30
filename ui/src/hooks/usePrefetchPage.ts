import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import { getGetRepoPageQueryOptions } from "@/api/__generated__/docz-api";

/*
 * Hover/focus prefetch for published-page links, mirroring
 * usePrefetchDoc. The published path is percent-encoded as ONE segment
 * — the same spelling the reader requests, so the cache keys match.
 */

const PREFETCH_STALE_MS = 30_000;

export function usePrefetchPage(): (
  owner: string,
  name: string,
  path: string,
) => void {
  const queryClient = useQueryClient();
  // Stable identity so callers can list it as an effect dependency.
  return useCallback(
    (owner, name, path) => {
      void queryClient.prefetchQuery(
        getGetRepoPageQueryOptions(owner, name, encodeURIComponent(path), {
          query: { staleTime: PREFETCH_STALE_MS },
        }),
      );
    },
    [queryClient],
  );
}
