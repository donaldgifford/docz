import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

const REPOS_ENDPOINT = "*/api/v1/repos";

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

describe("repos grid", () => {
  it("renders a card per repo whose counts come from search facets", async () => {
    mountAt("/repos");

    const siteCard = await screen.findByRole("link", {
      name: /donaldgifford\/docz-site/,
    });
    await waitFor(() => {
      expect(siteCard.textContent).toContain("docs: 2");
    });
    expect(siteCard.textContent).toContain("1 design");
    expect(siteCard.textContent).toContain("1 impl");
    expect(siteCard.textContent).toContain("main");
    expect(siteCard).toHaveAttribute("href", "/donaldgifford/docz-site");

    const apiCard = screen.getByRole("link", {
      name: /donaldgifford\/docz-api/,
    });
    await waitFor(() => {
      expect(apiCard.textContent).toContain("2 design");
    });
    // last_synced_sha rendered short (no last-updated in the contract).
    expect(apiCard.textContent).toContain("sync fixture");
  });

  it("shows skeleton cards before the list resolves", async () => {
    mountAt("/repos");

    await waitFor(() => {
      expect(screen.getByTestId("repos-skeleton")).toBeInTheDocument();
    });
    await screen.findByRole("link", { name: /donaldgifford\/docz-site/ });
    expect(screen.queryByTestId("repos-skeleton")).not.toBeInTheDocument();
  });

  it("shows the onboarding empty state when no repos exist", async () => {
    server.use(
      http.get(REPOS_ENDPOINT, () => HttpResponse.json({ repos: [] })),
    );
    mountAt("/repos");

    expect(await screen.findByText("No repositories yet")).toBeInTheDocument();
  });

  it("renders an inline error whose retry recovers", async () => {
    let failing = true;
    server.use(
      http.get(REPOS_ENDPOINT, () =>
        failing
          ? HttpResponse.json({ error: "boom" }, { status: 500 })
          : undefined,
      ),
    );
    const user = userEvent.setup();
    mountAt("/repos");

    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();

    failing = false;
    await user.click(screen.getByRole("button", { name: "retry" }));
    expect(
      await screen.findByRole("link", { name: /donaldgifford\/docz-site/ }),
    ).toBeInTheDocument();
  });

  it("renders the bare session panel on 401", async () => {
    server.use(
      http.get(REPOS_ENDPOINT, () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt("/repos");

    expect(await screen.findByText("Session required")).toBeInTheDocument();
  });
});
