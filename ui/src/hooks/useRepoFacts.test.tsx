import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useRepoFacts } from "@/hooks/useRepoFacts";

import type { ReactNode } from "react";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useRepoFacts", () => {
  it("returns the repo total and per-type counts from search facets", async () => {
    const { result } = renderHook(
      () => useRepoFacts("donaldgifford/docz-site"),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.facts).toBeDefined();
    });
    expect(result.current.facts).toEqual({
      total: 2,
      typeCounts: { design: 1, impl: 1 },
    });
  });
});
