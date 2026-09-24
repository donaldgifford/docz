import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

/*
 * Changelog page states (DESIGN-0002 Component 3). Render memoization
 * is delegated to useRenderedSource — its own suite pins "pipeline
 * runs once per (id, hash)", and this page keys with
 * (repo-changelog:<repoId>, changelog_sha).
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

const CHANGELOG_404 = http.get("*/api/v1/repos/:owner/:name/changelog", () =>
  HttpResponse.json({ error: "changelog not found" }, { status: 404 }),
);

describe("repo changelog", () => {
  it("renders the real changelog with its own h1 and a version ToC", async () => {
    mountAt("/donaldgifford/docz-site/changelog");

    // The file's h1 is kept — it IS the page title (INV-0005 OQ-3a).
    expect(
      await screen.findByRole("heading", { level: 1, name: "Changelog" }),
    ).toBeInTheDocument();

    // The fixture is this repo's real CHANGELOG.md; 0.1.0 is permanent
    // history, so it always has a version heading and a ToC jump link.
    expect(
      await screen.findByRole("heading", { level: 2, name: /0\.1\.0/ }),
    ).toBeInTheDocument();
    const versionLinks = screen.getAllByRole("link", { name: /0\.1\.0/ });
    expect(
      versionLinks.some((link) =>
        (link.getAttribute("href") ?? "").startsWith("#"),
      ),
    ).toBe(true);

    // Breadcrumbs: repo crumb links home, current crumb is static.
    expect(
      screen.getByRole("link", { name: "donaldgifford/docz-site" }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site");
  });

  it("shows the empty state for an empty-but-present changelog", async () => {
    mountAt("/donaldgifford/docz-api/changelog");

    expect(await screen.findByText(/is empty at HEAD/)).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Changelog" }),
    ).toBeInTheDocument();
  });

  it("renders the quiet absent-at-HEAD panel when config is enabled but the file 404s", async () => {
    server.use(CHANGELOG_404);
    mountAt("/donaldgifford/docz-site/changelog");

    // Fixture config enables changelog: the panel names the file (in a
    // code span — the RepoNav hint also says CHANGELOG.md, twice).
    expect(await screen.findByText(/absent at HEAD/)).toBeInTheDocument();
    expect(
      screen.getByText("CHANGELOG.md", { selector: "code" }),
    ).toBeInTheDocument();
  });

  it("renders the quiet not-enabled panel when the config has no changelog block", async () => {
    server.use(
      CHANGELOG_404,
      http.get("*/api/v1/repos/:owner/:name", () =>
        HttpResponse.json({
          repo: "donaldgifford/docz-site",
          default_branch: "main",
          docs_dir: "docs",
          last_synced_sha: "fixture-head-docz-site",
          config_snapshot: { docs_dir: "docs" },
          types: [],
        }),
      ),
    );
    mountAt("/donaldgifford/docz-site/changelog");

    expect(
      await screen.findByText(/doesn.t serve a changelog/),
    ).toBeInTheDocument();
  });
});
