import { QueryClient } from "@tanstack/react-query";

import { NotFoundError, SessionRequiredError } from "@/api/fetcher";

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Docs change on ingest cadence, not per keystroke — a short
        // staleTime keeps route hops from refetching identical data.
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          // 401/404 are stable answers, not transient faults.
          if (
            error instanceof SessionRequiredError ||
            error instanceof NotFoundError
          ) {
            return false;
          }
          return failureCount < 2;
        },
      },
    },
  });
}
