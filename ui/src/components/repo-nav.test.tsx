import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { RepoNav } from "@/components/repo-nav";
import { server } from "@/test/server";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";

function mountNavAt(path: string) {
  // The real route shape matters: RepoNav reads `:type` from useParams
  // to decide which drawer follows navigation.
  const router = createMemoryRouter(
    [
      {
        path: "/:owner/:repo/:type?/:docId?",
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
  it("renders identity, Home, and collapsed typed sections with counts", async () => {
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

    // Drawers start closed on non-type routes — no doc links yet.
    expect(
      screen.queryByRole("link", {
        name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
      }),
    ).not.toBeInTheDocument();
  });

  it("opens and closes a type drawer from the caret toggle", async () => {
    const user = userEvent.setup();
    mountNavAt("/donaldgifford/docz-site");

    const toggle = await screen.findByRole("button", {
      name: "design documents",
    });
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const docLink = await screen.findByRole("link", {
      name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
    });
    expect(docLink).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/design/DESIGN-0001",
    );

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByRole("link", {
        name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
      }),
    ).not.toBeInTheDocument();
  });

  it("disables the toggle for empty types", async () => {
    mountNavAt("/donaldgifford/docz-site");

    // docz-site's investigation type has no docs in the fixtures.
    const toggle = await screen.findByRole("button", {
      name: "investigation documents",
    });
    await waitFor(() => {
      expect(toggle).toBeDisabled();
    });
  });

  it("prefetches a doc when its nav link is hovered", async () => {
    let docRequests = 0;
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs/:docId", () => {
        docRequests += 1;
        return undefined; // fall through to the fixture handler
      }),
    );
    const user = userEvent.setup();
    mountNavAt("/donaldgifford/docz-site");
    await user.click(
      await screen.findByRole("button", { name: "design documents" }),
    );
    const docLink = await screen.findByRole("link", {
      name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
    });

    expect(docRequests).toBe(0);
    await user.hover(docLink);
    await waitFor(() => {
      expect(docRequests).toBe(1);
    });
  });

  it("auto-expands the active type and marks the open doc active", async () => {
    mountNavAt("/donaldgifford/docz-site/design/DESIGN-0001");

    // The design drawer follows the route — no toggle needed.
    const docLink = await screen.findByRole("link", {
      name: `DESIGN-0001 · ${SITE_DESIGN_TITLE}`,
    });
    await waitFor(() => {
      expect(docLink).toHaveAttribute("aria-current", "page");
    });
    // Sibling types stay closed.
    expect(
      screen.queryByRole("link", { name: /IMPL-0001/ }),
    ).not.toBeInTheDocument();

    // Home matches exactly, so it is NOT active on a doc page.
    expect(screen.getByRole("link", { name: /Home/ })).not.toHaveAttribute(
      "aria-current",
    );
  });
});
