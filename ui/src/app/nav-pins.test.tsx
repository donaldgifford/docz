import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { routes } from "@/app/router";

/*
 * Topbar nav pins (DESIGN-0002 Component 1): the deployment-injected
 * link list renders between Repos and the session menu and navigates
 * client-side. No config → no pins, and the topbar is unchanged.
 */

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

afterEach(() => {
  delete window.__DOCZ_CONFIG__;
});

describe("topbar nav pins", () => {
  it("renders injected pins between Repos and the session menu", async () => {
    window.__DOCZ_CONFIG__ = {
      nav: [
        { label: "RFCs", href: "/donaldgifford/docz-api/rfc" },
        { label: "Team Docs", href: "/donaldgifford/docz-site" },
      ],
    };
    mountAt("/repos");

    const nav = within(await screen.findByRole("navigation"));
    const links = nav.getAllByRole("link");
    const labels = links.map((link) => link.textContent);
    expect(labels).toEqual(["Directory", "Repos", "RFCs", "Team Docs"]);
    expect(nav.getByRole("link", { name: "RFCs" })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-api/rfc",
    );
  });

  it("navigates client-side through a pin", async () => {
    window.__DOCZ_CONFIG__ = {
      nav: [{ label: "RFCs", href: "/donaldgifford/docz-api/rfc" }],
    };
    const router = mountAt("/repos");
    await screen.findByRole("link", { name: "RFCs" });

    await userEvent.click(screen.getByRole("link", { name: "RFCs" }));
    // Lazy route: the heading arriving proves the navigation completed.
    expect(
      await screen.findByRole("heading", { level: 1, name: "RFCs" }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/donaldgifford/docz-api/rfc");
  });

  it("renders no pins without config", async () => {
    mountAt("/repos");
    const nav = within(await screen.findByRole("navigation"));
    expect(nav.getAllByRole("link").map((link) => link.textContent)).toEqual([
      "Directory",
      "Repos",
    ]);
  });
});
