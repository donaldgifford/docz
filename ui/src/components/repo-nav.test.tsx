import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { RepoNav } from "@/components/repo-nav";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";

function mountNavAt(path: string) {
  const router = createMemoryRouter(
    [
      {
        path: "*",
        element: <RepoNav owner="donaldgifford" name="docz-site" />,
      },
    ],
    { initialEntries: [path] },
  );
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe("RepoNav", () => {
  it("renders identity, Home, typed sections with counts, nested docs", async () => {
    mountNavAt("/donaldgifford/docz-site");

    expect(
      await screen.findByText("donaldgifford/docz-site"),
    ).toBeInTheDocument();
    expect(await screen.findByText("main · docz.yaml")).toBeInTheDocument();

    const home = screen.getByRole("link", { name: /Home/ });
    expect(home).toHaveAttribute("href", "/donaldgifford/docz-site");

    // Type items carry the facet count; links use the canonical name.
    const design = await screen.findByRole("link", { name: /^design 1$/ });
    expect(design).toHaveAttribute("href", "/donaldgifford/docz-site/design");
    expect(screen.getByRole("link", { name: /^impl 1$/ })).toBeInTheDocument();

    // Docs nest beneath their type as "ID · Title" links.
    const docLink = await screen.findByRole("link", {
      name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
    });
    expect(docLink).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/design/DESIGN-0001",
    );
  });

  it("marks the open doc and its type as active", async () => {
    mountNavAt("/donaldgifford/docz-site/design/DESIGN-0001");

    const docLink = await screen.findByRole("link", {
      name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
    });
    await waitFor(() => {
      expect(docLink).toHaveAttribute("aria-current", "page");
    });

    // Home matches exactly, so it is NOT active on a doc page.
    expect(screen.getByRole("link", { name: /Home/ })).not.toHaveAttribute(
      "aria-current",
    );
  });
});
