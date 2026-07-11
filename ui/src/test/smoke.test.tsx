import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { useListRepos } from "@/api/__generated__/docz-api";
import { routes } from "@/app/router";

function testQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

describe("app smoke", () => {
  it("renders the shell and the directory route", async () => {
    const router = createMemoryRouter(routes, { initialEntries: ["/"] });
    render(
      <QueryClientProvider client={testQueryClient()}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("docz")).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", { name: "Directory" }),
    ).toBeInTheDocument();
  });

  it("resolves a generated hook against the MSW server", async () => {
    function wrapper({ children }: { children: ReactNode }) {
      return (
        <QueryClientProvider client={testQueryClient()}>
          {children}
        </QueryClientProvider>
      );
    }

    const { result } = renderHook(() => useListRepos(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    const response = result.current.data;
    if (response?.status !== 200) {
      throw new Error(`expected a 200 envelope, got ${String(response?.status)}`);
    }
    expect(response.data.repos).toBeDefined();
  });
});
