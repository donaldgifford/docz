import { QueryClient } from "@tanstack/react-query";

import {
  BadRequestError,
  NotFoundError,
  SessionRequiredError,
} from "@/api/fetcher";

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Docs change on ingest cadence, not per keystroke — a short
        // staleTime keeps route hops from refetching identical data.
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          // 400/401/404 are stable answers, not transient faults —
          // asking again gets the same reply.
          if (
            error instanceof BadRequestError ||
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
