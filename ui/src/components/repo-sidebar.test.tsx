import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { delay, http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

/*
 * A repository's groups in the shell's sidebar (DESIGN-0028 §6). The
 * rows are the design system's tree, so what these hold is what docz
 * hands it: which rows a repository gets, what each links to, when a
 * type's documents are fetched, and which branch the route opens.
 *
 * The whole app is mounted, not the groups alone: which repository,
 * type, document, and page is current comes from AppShell reading the
 * route in force, and that wiring is half of what is under test.
 */

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";
const SITE_DESIGN_DOC = `DESIGN-0001 · ${SITE_DESIGN_TITLE}`;

async function mountAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const nav = within(
    await screen.findByRole("navigation", { name: "docz navigation" }),
  );
  // The repository's groups are a lazy chunk: Home is their arrival.
  await nav.findByRole("link", { name: "Home" });
  return { nav, queryClient };
}

/** Every request the mount started has answered. */
async function settled(queryClient: QueryClient): Promise<void> {
  await waitFor(() => {
    expect(queryClient.isFetching()).toBe(0);
  });
}

/** The row (its list item) that holds a link or a button. */
function rowOf(control: HTMLElement): HTMLElement {
  const row = control.closest("li");
  if (row === null) {
    throw new Error("the control is not in a sidebar row");
  }
  return row;
}

describe("the repository's groups", () => {
  it("names the repository and lists Home and a closed row per type, with counts", async () => {
    const { nav } = await mountAt("/donaldgifford/docz-site");

    expect(nav.getByText("donaldgifford/docz-site")).toBeInTheDocument();
    const home = nav.getByRole("link", { name: "Home" });
    expect(home).toHaveAttribute("href", "/donaldgifford/docz-site");
    expect(home).toHaveAttribute("aria-current", "page");

    // A type's row links to its index by the canonical name, and
    // carries the facet count beside the link.
    const design = await nav.findByRole("link", { name: "design" });
    expect(design).toHaveAttribute("href", "/donaldgifford/docz-site/design");
    await waitFor(() => {
      expect(rowOf(design)).toHaveTextContent(/^design1$/);
    });
    expect(nav.getByRole("link", { name: "impl" })).toBeInTheDocument();

    // Rows start closed off a type's routes: no document links yet.
    expect(
      nav.queryByRole("link", { name: SITE_DESIGN_DOC }),
    ).not.toBeInTheDocument();
  });

  it("opens and closes a type's row from its caret", async () => {
    const user = userEvent.setup();
    const { nav } = await mountAt("/donaldgifford/docz-site");

    const caret = await nav.findByRole("button", {
      name: "design, show contents",
    });
    expect(caret).toHaveAttribute("aria-expanded", "false");

    await user.click(caret);
    expect(caret).toHaveAttribute("aria-expanded", "true");
    expect(
      await nav.findByRole("link", { name: SITE_DESIGN_DOC }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site/design/DESIGN-0001");

    await user.click(caret);
    expect(caret).toHaveAttribute("aria-expanded", "false");
    await waitFor(() => {
      expect(
        nav.queryByRole("link", { name: SITE_DESIGN_DOC }),
      ).not.toBeInTheDocument();
    });
  });

  // The route beside the sidebar lists every type's documents for its
  // cross-reference index, so a request count cannot tell whose it was.
  // That a row fetches when it opens shows as the line it draws
  // meanwhile.
  it("says a row's documents are loading, in a line of the list", async () => {
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/design/docs", async () => {
        await delay(150);
        return undefined;
      }),
    );
    const user = userEvent.setup();
    const { nav } = await mountAt("/donaldgifford/docz-site");

    await user.click(
      await nav.findByRole("button", { name: "design, show contents" }),
    );
    const loading = await nav.findByRole("status");
    expect(loading).toHaveTextContent("Loading…");
    // A paragraph is not a list's child: the note is the list item.
    expect(loading.parentElement?.tagName).toBe("LI");

    expect(
      await nav.findByRole("link", { name: SITE_DESIGN_DOC }),
    ).toBeInTheDocument();
    expect(nav.queryByRole("status")).not.toBeInTheDocument();
  });

  it("keeps the caret of an empty type shut", async () => {
    const { nav } = await mountAt("/donaldgifford/docz-site");

    // docz-site's investigation type has no docs in the fixtures.
    const caret = await nav.findByRole("button", {
      name: "investigation, show contents",
    });
    await waitFor(() => {
      expect(caret).toBeDisabled();
    });
  });

  it("fetches a type's documents from the icon rail's popover too", async () => {
    const user = userEvent.setup();
    const { nav } = await mountAt("/donaldgifford/docz-site");
    await nav.findByRole("link", { name: "design" });

    // Collapsed to the rail, a type's row is one button; its popover
    // holds the type's index and its documents.
    // ⌘B is the package's shortcut for it, on the window.
    await user.keyboard("{Meta>}b{/Meta}");
    await user.click(await nav.findByRole("button", { name: "design" }));

    const popover = within(
      await screen.findByRole("dialog", { name: "design" }),
    );
    expect(popover.getByRole("link", { name: "design" })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/design",
    );
    expect(
      await popover.findByRole("link", { name: SITE_DESIGN_DOC }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site/design/DESIGN-0001");
  });

  it("prefetches a document when its row is hovered", async () => {
    let docRequests = 0;
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs/:docId", () => {
        docRequests += 1;
        return undefined;
      }),
    );
    const user = userEvent.setup();
    const { nav } = await mountAt("/donaldgifford/docz-site");
    await user.click(
      await nav.findByRole("button", { name: "design, show contents" }),
    );
    const doc = await nav.findByRole("link", { name: SITE_DESIGN_DOC });

    expect(docRequests).toBe(0);
    await user.hover(doc);
    await waitFor(() => {
      expect(docRequests).toBe(1);
    });
  });

  it("opens the route's type and marks the open document current", async () => {
    const { nav } = await mountAt(
      "/donaldgifford/docz-site/design/DESIGN-0001",
    );

    // The design row follows the route — no toggle needed.
    const doc = await nav.findByRole("link", { name: SITE_DESIGN_DOC });
    expect(doc).toHaveAttribute("aria-current", "page");
    // The type's own row is its index, which is not this page.
    expect(nav.getByRole("link", { name: "design" })).not.toHaveAttribute(
      "aria-current",
    );
    // Sibling types stay closed.
    expect(
      nav.queryByRole("link", { name: /IMPL-0001/ }),
    ).not.toBeInTheDocument();
    // Home matches exactly, so it is NOT current on a document.
    expect(nav.getByRole("link", { name: "Home" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("marks the type's row current on its index, by an alias too", async () => {
    const { nav } = await mountAt("/donaldgifford/docz-site/rb");

    const runbook = await nav.findByRole("link", { name: "runbook" });
    await waitFor(() => {
      expect(runbook).toHaveAttribute("aria-current", "page");
    });
    expect(
      nav.getByRole("button", { name: "runbook, show contents" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("offers no groups for a repository docz-api does not know", async () => {
    server.use(
      http.get("*/api/v1/repos/nobody/nothing", () =>
        HttpResponse.json({ error: "repo not found" }, { status: 404 }),
      ),
    );
    const router = createMemoryRouter(routes, {
      initialEntries: ["/nobody/nothing"],
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    const nav = within(
      await screen.findByRole("navigation", { name: "docz navigation" }),
    );

    await settled(queryClient);
    await waitFor(() => {
      expect(nav.queryByRole("link", { name: "Home" })).not.toBeInTheDocument();
    });
    expect(nav.getByRole("link", { name: "Directory" })).toBeInTheDocument();
  });
});

describe("the pages tree", () => {
  const GUIDE_TITLE = "Local development against a real docz-api";

  it("draws the opted repo's tree: leaf titles, directory-page links, closed directories", async () => {
    const { nav } = await mountAt("/donaldgifford/docz-site");

    expect(await nav.findByText("Pages")).toBeInTheDocument();

    // File pages at the top level link with their PageSummary titles.
    expect(nav.getByRole("link", { name: "docz-site" })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/README.md",
    );
    // Directory pages (extensionless) are links too.
    expect(nav.getByRole("link", { name: "Design Documents" })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/design",
    );

    // guides/ has no page of its own: a toggle, never a link — and its
    // children are not drawn until it opens.
    expect(nav.getByRole("button", { name: "guides/" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    expect(
      nav.queryByRole("link", { name: GUIDE_TITLE }),
    ).not.toBeInTheDocument();
  });

  it("opens a directory from its row without navigating", async () => {
    const user = userEvent.setup();
    const { nav } = await mountAt("/donaldgifford/docz-site");

    const row = await nav.findByRole("button", { name: "guides/" });
    await user.click(row);
    expect(row).toHaveAttribute("aria-expanded", "true");
    expect(await nav.findByRole("link", { name: GUIDE_TITLE })).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/guides/local-dev.md",
    );

    await user.click(row);
    await waitFor(() => {
      expect(
        nav.queryByRole("link", { name: GUIDE_TITLE }),
      ).not.toBeInTheDocument();
    });
  });

  it("opens the route's branch and marks the page current", async () => {
    const { nav } = await mountAt(
      "/donaldgifford/docz-site/pages/guides/local-dev.md",
    );

    const leaf = await nav.findByRole("link", { name: GUIDE_TITLE });
    expect(leaf).toHaveAttribute("aria-current", "page");
  });

  it("prefetches a page on hover with the one-segment encoded spelling", async () => {
    const requested: string[] = [];
    server.use(
      http.get("*/api/v1/repos/:owner/:name/pages/*", ({ request }) => {
        requested.push(new URL(request.url).pathname);
        return undefined;
      }),
    );
    const user = userEvent.setup();
    const { nav } = await mountAt("/donaldgifford/docz-site");

    await user.click(await nav.findByRole("button", { name: "guides/" }));
    const leaf = await nav.findByRole("link", { name: GUIDE_TITLE });

    const wanted = (path: string): boolean =>
      path.endsWith("/pages/guides%2Flocal-dev.md");
    expect(requested.some(wanted)).toBe(false);
    await user.hover(leaf);
    await waitFor(() => {
      expect(requested.some(wanted)).toBe(true);
    });
  });

  it("stays out for a repo that has not opted in, with zero pages requests", async () => {
    let pagesRequests = 0;
    server.use(
      http.get("*/api/v1/repos/:owner/:name/pages", () => {
        pagesRequests += 1;
        return undefined;
      }),
    );
    const { nav, queryClient } = await mountAt("/donaldgifford/docz-api");

    await nav.findByText("Documents");
    await settled(queryClient);
    expect(nav.queryByText("Pages")).not.toBeInTheDocument();
    expect(pagesRequests).toBe(0);
  });
});

describe("the changelog row", () => {
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

  it("sits under Home when the snapshot enables the changelog block", async () => {
    const { nav } = await mountAt("/donaldgifford/docz-site");

    const link = await nav.findByRole("link", { name: "Changelog" });
    expect(link).toHaveAttribute("href", "/donaldgifford/docz-site/changelog");
    // The fixture's file is a bare basename: the hint, and no tooltip
    // beyond the row's own label.
    expect(rowOf(link)).toHaveTextContent("CHANGELOG.md");
    expect(link).not.toHaveAttribute("title");
  });

  it("shows the basename as its hint and the full subpath as its tooltip", async () => {
    server.use(
      repoWithSnapshot({
        changelog: { enabled: true, file: "charts/docz-site/CHANGELOG.md" },
      }),
    );
    const { nav } = await mountAt("/donaldgifford/docz-site");

    const link = await nav.findByRole("link", { name: "Changelog" });
    expect(rowOf(link)).toHaveTextContent(/^ChangelogCHANGELOG\.md$/);
    expect(link).toHaveAttribute("title", "charts/docz-site/CHANGELOG.md");
  });

  it.each([
    ["no changelog block", { docs_dir: "docs" }],
    ["a malformed changelog block", { changelog: { enabled: "true" } }],
  ])("stays out with %s", async (_, snapshot) => {
    server.use(repoWithSnapshot(snapshot));
    const { nav, queryClient } = await mountAt("/donaldgifford/docz-site");

    await settled(queryClient);
    expect(
      nav.queryByRole("link", { name: "Changelog" }),
    ).not.toBeInTheDocument();
  });

  it("prefetches the changelog on hover", async () => {
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
    const { nav } = await mountAt("/donaldgifford/docz-site");

    const link = await nav.findByRole("link", { name: "Changelog" });
    expect(changelogRequests).toBe(0);
    await user.hover(link);
    await waitFor(() => {
      expect(changelogRequests).toBe(1);
    });
  });
});
