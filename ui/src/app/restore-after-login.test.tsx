import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

const RETURN_KEY = "docz:auth:return-to";

function mountAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

/*
 * The 401 → /login → OAuth → "/" round trip, from the landing side.
 * The stash half is covered in each route's 401 test; these cover
 * RestoreAfterLogin in AppShell. getSession 200 comes from the
 * generated faker handler (identity content doesn't matter here).
 */
describe("destination restore after login", () => {
  it("replaces the callback landing with the stashed destination", async () => {
    sessionStorage.setItem(RETURN_KEY, "/repos");
    const router = mountAt("/");

    expect(
      await screen.findByRole(
        "heading",
        { name: "Repositories" },
        { timeout: 10_000 },
      ),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/repos");
    expect(sessionStorage.getItem(RETURN_KEY)).toBeNull();
  });

  it("leaves the stash for the next login when signed out", async () => {
    server.use(
      http.get("*/api/v1/auth/session", () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    sessionStorage.setItem(RETURN_KEY, "/repos");
    const router = mountAt("/");

    // the directory still renders; no navigation happens
    await screen.findByTestId("results-count", undefined, { timeout: 10_000 });
    await waitFor(() => {
      expect(sessionStorage.getItem(RETURN_KEY)).toBe("/repos");
    });
    expect(router.state.location.pathname).toBe("/");
  });
});
