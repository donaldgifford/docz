import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { routes } from "@/app/router";

/*
 * Nav pins (DESIGN-0002 Component 1, DESIGN-0028 §6): the
 * deployment-injected link list is the sidebar's Pinned group, under
 * Browse, and navigates client-side. No config → no pins and no group.
 */

/** The sidebar's own links, without the mark's link home at its head. */
function sidebarLinks(nav: HTMLElement): string[] {
  return within(nav)
    .getAllByRole("link")
    .map((link) => link.textContent)
    .filter((label) => label !== "doczreader");
}

async function findSidebar(): Promise<HTMLElement> {
  return screen.findByRole("navigation", { name: "docz navigation" });
}

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

describe("nav pins", () => {
  it("lists injected pins in a Pinned group under Browse", async () => {
    window.__DOCZ_CONFIG__ = {
      nav: [
        { label: "RFCs", href: "/donaldgifford/docz-api/rfc" },
        { label: "Team Docs", href: "/donaldgifford/docz-site" },
      ],
    };
    mountAt("/repos");

    const nav = await findSidebar();
    // A pin with no icon leads with its first letter, for the rail.
    expect(sidebarLinks(nav)).toEqual([
      "Directory",
      "Repositories",
      "RRFCs",
      "TTeam Docs",
    ]);
    expect(within(nav).getByText("Pinned")).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: "RFCs" })).toHaveAttribute(
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
    // The pin is the current page now, and Repositories no longer is.
    expect(screen.getByRole("link", { name: "RFCs" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("link", { name: "Repositories" }),
    ).not.toHaveAttribute("aria-current");
  });

  it("renders no pins without config", async () => {
    mountAt("/repos");
    const nav = await findSidebar();
    expect(sidebarLinks(nav)).toEqual(["Directory", "Repositories"]);
    expect(within(nav).queryByText("Pinned")).not.toBeInTheDocument();
  });
});
