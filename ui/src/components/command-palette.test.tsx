import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { createQueryClient } from "@/app/query-client";
import { routes } from "@/app/router";
import { server } from "@/test/server";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";
const SITE_IMPL_TITLE =
  "docz-site MVP: phased build of the reader, directory, and repo pages";
const API_DESIGN_TITLE =
  "docz-api cross-repo docz registry and ingestion service";

function mountAt(path: string, queryClient?: QueryClient) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  queryClient ??= new QueryClient({
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
 * The palette is the design system's CommandDialog; what is held here
 * is docz's: the shortcuts, the query and its filters, the recents, the
 * prefetch, and where a hit goes. The palette's chunk loads on first
 * use, so finding the dialog is always a wait.
 */

const PALETTE = { name: "Search documents" } as const;

async function findPalette() {
  const dialog = await screen.findByRole("dialog", PALETTE);
  // The dialog hands focus to its input once it is open; a key pressed
  // before that goes to the page behind it.
  await waitFor(() => {
    expect(within(dialog).getByRole("combobox")).toHaveFocus();
  });
  return dialog;
}

async function palette() {
  return within(await findPalette());
}

async function expectPaletteClosed() {
  await waitFor(() => {
    expect(screen.queryByRole("dialog", PALETTE)).not.toBeInTheDocument();
  });
}

describe("command palette", () => {
  it("opens with ⌘K on any route and closes with Esc", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    expect(screen.queryByRole("dialog", PALETTE)).not.toBeInTheDocument();
    await user.keyboard("{Meta>}k{/Meta}");
    expect(await findPalette()).toBeInTheDocument();

    await user.keyboard("{Escape}");
    await expectPaletteClosed();
  });

  it("opens with / — except while typing in an editable element", async () => {
    const user = userEvent.setup();
    mountAt("/");
    const box = await screen.findByRole("searchbox", {
      name: "Search documents",
    });

    // Typing "/" into the directory search box stays typing.
    await user.click(box);
    await user.keyboard("/");
    expect(screen.queryByRole("dialog", PALETTE)).not.toBeInTheDocument();
    expect(box).toHaveValue("/");

    // From the page body it opens the palette.
    await user.click(document.body);
    await user.keyboard("/");
    expect(await findPalette()).toBeInTheDocument();
  });

  it("groups results by repo and previews the highlighted hit", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();

    // Empty query lists everything, grouped per repo.
    expect(
      await dialog.findByText("donaldgifford/docz-site — 9 matches"),
    ).toBeInTheDocument();
    expect(
      dialog.getByText("donaldgifford/docz-api — 3 matches"),
    ).toBeInTheDocument();

    // The first hit is highlighted and previewed from hit data alone.
    const preview = dialog.getByRole("group", { name: "Preview" });
    await waitFor(() => {
      expect(within(preview).getByText(SITE_DESIGN_TITLE)).toBeInTheDocument();
    });

    // Tab steps the highlight (and the preview) to the next hit.
    await user.keyboard("{Tab}");
    await waitFor(() => {
      expect(within(preview).getByText(SITE_IMPL_TITLE)).toBeInTheDocument();
    });
  });

  it("filters as you type (debounced searchDocs)", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    await dialog.findByText("donaldgifford/docz-site — 9 matches");

    await user.keyboard("ingestion service");
    await waitFor(() => {
      expect(
        dialog.getByText("donaldgifford/docz-api — 1 match"),
      ).toBeInTheDocument();
    });
    // Present in the list and (as the highlighted hit) in the preview.
    expect(dialog.getAllByText(API_DESIGN_TITLE).length).toBeGreaterThan(0);
    expect(dialog.queryByText(SITE_IMPL_TITLE)).not.toBeInTheDocument();
  });

  it("narrows through repo and type pills applied to the palette only", async () => {
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    await dialog.findByText("donaldgifford/docz-site — 9 matches");

    // Repo pill (short name) scopes to that repo.
    await user.click(dialog.getByRole("button", { name: "docz-api" }));
    await waitFor(() => {
      expect(
        dialog.queryByText(/donaldgifford\/docz-site —/),
      ).not.toBeInTheDocument();
    });
    expect(
      dialog.getByText("donaldgifford/docz-api — 3 matches"),
    ).toBeInTheDocument();

    // Type pill replaces the repo pill.
    await user.click(dialog.getByRole("button", { name: "impl" }));
    await waitFor(() => {
      expect(
        dialog.getByText("donaldgifford/docz-site — 1 match"),
      ).toBeInTheDocument();
    });
    expect(dialog.getAllByText(SITE_IMPL_TITLE).length).toBeGreaterThan(0);

    // The pills never touched the page URL.
    expect(router.state.location.search).toBe("");
  });

  it("opens on the reader route too (mounted in the shell)", async () => {
    const user = userEvent.setup();
    mountAt("/donaldgifford/docz-site/design/DESIGN-0001");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    expect(await findPalette()).toBeInTheDocument();
  });

  it("finds documents by body text, not just titles", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    await dialog.findByText("donaldgifford/docz-site — 9 matches");

    // "frontmatter" appears in doc bodies, never in a fixture title.
    await user.keyboard("frontmatter");
    await waitFor(() => {
      expect(dialog.getAllByText(/ — \d+ match/).length).toBeGreaterThan(0);
    });
    const titles = [SITE_DESIGN_TITLE, SITE_IMPL_TITLE, API_DESIGN_TITLE];
    expect(
      titles.some((title) => dialog.queryAllByText(title).length > 0),
    ).toBe(true);
  });

  it("opens from the topbar search affordance", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.click(
      screen.getByRole("button", { name: /Search docs, rfcs, authors/ }),
    );
    expect(await findPalette()).toBeInTheDocument();
  });

  it("opens the first hit on Enter without arrowing first", async () => {
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    await (await palette()).findByText("donaldgifford/docz-site — 9 matches");
    await user.keyboard("ingestion service");
    await (await palette()).findByText("donaldgifford/docz-api — 1 match");

    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-api/design/DESIGN-0001",
      );
    });
  });

  it("prefetches the highlighted hit's doc", async () => {
    const docRequests: string[] = [];
    server.use(
      http.get(
        "*/api/v1/repos/:owner/:name/types/:type/docs/:docId",
        ({ params }) => {
          docRequests.push(String(params.docId));
          return undefined; // fall through to the fixture handler
        },
      ),
    );
    const user = userEvent.setup();
    // The app's real client defaults (30s staleTime) — the point is
    // that the prefetch makes the reader mount a cache hit.
    mountAt("/repos", createQueryClient());
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    await (await palette()).findByText("donaldgifford/docz-site — 9 matches");

    // The auto-highlighted first hit warms immediately…
    await waitFor(() => {
      expect(docRequests).toContain("DESIGN-0001");
    });
    // …and stepping the highlight warms the next hit.
    await user.keyboard("{ArrowDown}");
    await waitFor(() => {
      expect(docRequests).toContain("IMPL-0001");
    });

    // Opening it renders from the prefetched cache — the reader
    // mounts, paints, and getDoc never fires a second time.
    await user.keyboard("{Enter}");
    await screen.findByRole(
      "heading",
      { level: 1, name: /docz-site MVP: phased build/ },
      { timeout: 10_000 },
    );
    expect(docRequests.filter((id) => id === "IMPL-0001")).toHaveLength(1);
  });

  it("leads the empty query with recent docs and opens them", async () => {
    localStorage.setItem(
      "docz:recent-docs",
      JSON.stringify([
        {
          kind: "doc",
          repo: "donaldgifford/docz-api",
          type: "design",
          docId: "DESIGN-0002",
          title: "OpenAPI contract for docz-api and the docz-site",
        },
      ]),
    );
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    expect(await dialog.findByText("recent")).toBeInTheDocument();

    // The recent entry is the initial highlight — Enter opens it.
    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-api/design/DESIGN-0002",
      );
    });
  });

  it("hides the recent group once a query is typed", async () => {
    localStorage.setItem(
      "docz:recent-docs",
      JSON.stringify([
        {
          kind: "doc",
          repo: "donaldgifford/docz-site",
          type: "impl",
          docId: "IMPL-0001",
          title: SITE_IMPL_TITLE,
        },
      ]),
    );
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    expect(await dialog.findByText("recent")).toBeInTheDocument();

    await user.keyboard("ingestion service");
    await waitFor(() => {
      expect(dialog.queryByText("recent")).not.toBeInTheDocument();
    });
  });

  it("renders page hits with the neutral marker and opens the page route", async () => {
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    // Page hits ride the docz-site group on the empty query — marker
    // line `page · <published path>`, no type badge.
    expect(
      await dialog.findByText("donaldgifford/docz-site — 9 matches"),
    ).toBeInTheDocument();
    expect(dialog.getByText("page · README.md")).toBeInTheDocument();

    // "container pair" only appears in the guide page's body.
    await user.keyboard("container pair");
    await dialog.findByText("donaldgifford/docz-site — 1 match");
    expect(dialog.getByText("page · guides/local-dev.md")).toBeInTheDocument();

    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-site/pages/guides/local-dev.md",
      );
    });
  });

  it("prefetches the highlighted page hit", async () => {
    const requested: string[] = [];
    server.use(
      http.get("*/api/v1/repos/:owner/:name/pages/*", ({ request }) => {
        requested.push(new URL(request.url).pathname);
        return undefined; // fall through to the fixture handler
      }),
    );
    const user = userEvent.setup();
    mountAt("/repos", createQueryClient());
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    await user.keyboard("container pair");
    await (await palette()).findByText("donaldgifford/docz-site — 1 match");

    // The lone page hit is auto-highlighted — the prefetch uses the
    // reader's one-segment encoded spelling, so the cache key matches.
    await waitFor(() => {
      expect(
        requested.some((path) => path.endsWith("/pages/guides%2Flocal-dev.md")),
      ).toBe(true);
    });
  });

  it("leads with page recents and opens them kind-aware", async () => {
    localStorage.setItem(
      "docz:recent-docs",
      JSON.stringify([
        {
          kind: "page",
          repo: "donaldgifford/docz-site",
          path: "guides/local-dev.md",
          title: "Local development against a real docz-api",
        },
      ]),
    );
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    expect(await dialog.findByText("recent")).toBeInTheDocument();
    // The marker line renders in the recent item and (as the
    // highlighted hit) in the preview pane.
    expect(
      dialog.getAllByText("page · guides/local-dev.md · docz-site").length,
    ).toBeGreaterThan(0);

    // The recent page entry is the initial highlight — Enter opens it.
    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-site/pages/guides/local-dev.md",
      );
    });
  });

  it("navigates to the reader on Enter and closes", async () => {
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    await (await palette()).findByText("donaldgifford/docz-site — 9 matches");

    // ↓ to the second hit, then Enter.
    await user.keyboard("{ArrowDown}{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-site/impl/IMPL-0001",
      );
    });
    await expectPaletteClosed();
  });

  it("closes when a hit is followed with the pointer, and navigates once", async () => {
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = await palette();
    await dialog.findByText("donaldgifford/docz-site — 9 matches");

    const navigate = vi.spyOn(router, "navigate");
    await user.click(dialog.getByRole("option", { name: /IMPL-0001/ }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-site/impl/IMPL-0001",
      );
    });
    await expectPaletteClosed();
    expect(navigate).toHaveBeenCalledOnce();
  });
});
