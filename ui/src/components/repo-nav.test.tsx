import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { RepoNav } from "@/components/repo-nav";
import { server } from "@/test/server";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";

function mountNavAt(path: string, name = "docz-site") {
  // The real route shape matters: RepoNav reads `:type` from useParams
  // to decide which drawer follows navigation, and the pages tree reads
  // the pages route's splat to auto-expand the active branch.
  const router = createMemoryRouter(
    [
      {
        path: "/:owner/:repo/pages/*",
        element: <RepoNav owner="donaldgifford" name={name} />,
      },
      {
        path: "/:owner/:repo/:type?/:docId?",
        element: <RepoNav owner="donaldgifford" name={name} />,
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

describe("RepoNav pages tree", () => {
  const GUIDE_TITLE = "Local development against a real docz-api";

  it("renders the opted repo's tree: leaf titles, directory-page links, closed dirs", async () => {
    mountNavAt("/donaldgifford/docz-site");

    expect(await screen.findByText("pages")).toBeInTheDocument();

    // File pages at the top level link with their PageSummary titles.
    expect(screen.getByRole("link", { name: "docz-site" })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/README.md",
    );
    // Directory pages (extensionless) are links too.
    expect(
      screen.getByRole("link", { name: "Design Documents" }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site/pages/design");

    // guides/ has no page of its own: a toggle, never a link — and its
    // children stay hidden until expanded.
    expect(screen.getByRole("button", { name: "guides/" })).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: GUIDE_TITLE }),
    ).not.toBeInTheDocument();
  });

  it("expands a directory from the caret without navigating", async () => {
    const user = userEvent.setup();
    mountNavAt("/donaldgifford/docz-site");

    const caret = await screen.findByRole("button", { name: "guides pages" });
    expect(caret).toHaveAttribute("aria-expanded", "false");

    await user.click(caret);
    expect(caret).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("link", { name: GUIDE_TITLE })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/guides/local-dev.md",
    );

    await user.click(caret);
    expect(
      screen.queryByRole("link", { name: GUIDE_TITLE }),
    ).not.toBeInTheDocument();
  });

  it("auto-expands the active route's branch and marks the page active", async () => {
    mountNavAt("/donaldgifford/docz-site/pages/guides/local-dev.md");

    const leaf = await screen.findByRole("link", { name: GUIDE_TITLE });
    await waitFor(() => {
      expect(leaf).toHaveAttribute("aria-current", "page");
    });
  });

  it("prefetches a page on hover with the one-segment encoded spelling", async () => {
    const requested: string[] = [];
    server.use(
      http.get("*/api/v1/repos/:owner/:name/pages/*", ({ request }) => {
        requested.push(new URL(request.url).pathname);
        return undefined; // fall through to the fixture handler
      }),
    );
    const user = userEvent.setup();
    mountNavAt("/donaldgifford/docz-site");

    await user.click(
      await screen.findByRole("button", { name: "guides pages" }),
    );
    const leaf = screen.getByRole("link", { name: GUIDE_TITLE });

    expect(requested).toEqual([]);
    await user.hover(leaf);
    await waitFor(() => {
      expect(
        requested.some((path) => path.endsWith("/pages/guides%2Flocal-dev.md")),
      ).toBe(true);
    });
  });

  it("stays hidden for non-opted repos with zero pages requests", async () => {
    let pagesRequests = 0;
    server.use(
      http.get("*/api/v1/repos/:owner/:name/pages", () => {
        pagesRequests += 1;
        return undefined;
      }),
    );
    mountNavAt("/donaldgifford/docz-api", "docz-api");

    await screen.findByText("main · docz.yaml");
    expect(screen.queryByText("pages")).not.toBeInTheDocument();
    expect(pagesRequests).toBe(0);
  });
});

describe("RepoNav changelog row", () => {
  function repoWithSnapshot(snapshot: Record<string, unknown>) {
    return http.get("*/api/v1/repos/:owner/:name", () =>
      HttpResponse.json({
        repo: "donaldgifford/docz-site",
        default_branch: "main",
        docs_dir: "docs",
        last_synced_sha: "fixture-head-docz-site",
        config_snapshot: snapshot,
        types: [],
      }),
    );
  }

  it("renders under Home when the snapshot enables the changelog block", async () => {
    mountNavAt("/donaldgifford/docz-site");

    const row = await screen.findByRole("link", { name: /Changelog/ });
    expect(row).toHaveAttribute("href", "/donaldgifford/docz-site/changelog");
    // Fixture file is a bare basename — hint only, no tooltip.
    expect(row).toHaveTextContent("CHANGELOG.md");
    expect(row).not.toHaveAttribute("title");
  });

  it("shows the basename hint with the full subpath as tooltip", async () => {
    server.use(
      repoWithSnapshot({
        changelog: { enabled: true, file: "charts/docz-site/CHANGELOG.md" },
      }),
    );
    mountNavAt("/donaldgifford/docz-site");

    const row = await screen.findByRole("link", { name: /Changelog/ });
    expect(row).toHaveTextContent("CHANGELOG.md");
    expect(row).toHaveAttribute("title", "charts/docz-site/CHANGELOG.md");
  });

  it("stays hidden without an enabled changelog block", async () => {
    server.use(repoWithSnapshot({ docs_dir: "docs" }));
    mountNavAt("/donaldgifford/docz-site");

    await screen.findByText("main · docz.yaml");
    expect(
      screen.queryByRole("link", { name: /Changelog/ }),
    ).not.toBeInTheDocument();
  });

  it("reads a malformed changelog block as hidden", async () => {
    server.use(repoWithSnapshot({ changelog: { enabled: "true" } }));
    mountNavAt("/donaldgifford/docz-site");

    await screen.findByText("main · docz.yaml");
    expect(
      screen.queryByRole("link", { name: /Changelog/ }),
    ).not.toBeInTheDocument();
  });

  it("prefetches the changelog on row hover", async () => {
    const user = userEvent.setup();
    let changelogRequests = 0;
    server.use(
      http.get("*/api/v1/repos/:owner/:name/changelog", () => {
        changelogRequests += 1;
        return HttpResponse.json({
          repo: "donaldgifford/docz-site",
          changelog_md: "# Changelog",
          changelog_sha: "fixture-sha",
        });
      }),
    );
    mountNavAt("/donaldgifford/docz-site");

    const row = await screen.findByRole("link", { name: /Changelog/ });
    expect(changelogRequests).toBe(0);
    await user.hover(row);
    await waitFor(() => {
      expect(changelogRequests).toBe(1);
    });
  });
});
