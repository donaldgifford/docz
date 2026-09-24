import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import { getGetRepoChangelogQueryOptions } from "@/api/__generated__/docz-api";

/*
 * Hover/focus prefetch for the RepoNav changelog row, mirroring
 * usePrefetchDoc: by the time the click lands the body is usually
 * cached and the page paints without a skeleton. staleTime only
 * applies to the prefetch — the mounted query revalidates per its own
 * defaults.
 */

const PREFETCH_STALE_MS = 30_000;

export function usePrefetchChangelog(): (owner: string, name: string) => void {
  const queryClient = useQueryClient();
  return useCallback(
    (owner, name) => {
      void queryClient.prefetchQuery(
        getGetRepoChangelogQueryOptions(owner, name, {
          query: { staleTime: PREFETCH_STALE_MS },
        }),
      );
    },
    [queryClient],
  );
}
