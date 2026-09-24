import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

/*
 * Published-page reader states (DESIGN-0004 Component 4) plus the
 * route-level guarantees: the reserved `pages` segment outranks
 * `:type`, the wire carries the splat as ONE percent-encoded segment,
 * doc↔page relative links resolve both directions through
 * useRepoDocIndex, a page never links its own source path, and
 * non-opted repos fire zero pages requests.
 */

const PAGE_URL = "/donaldgifford/docz-site/pages/guides/local-dev.md";
const PAGES_ENDPOINT = "*/api/v1/repos/:owner/:name/pages/*";

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

async function findRenderedGuide() {
  return await screen.findByRole(
    "heading",
    { level: 1, name: "Local development against a real docz-api" },
    { timeout: 10_000 },
  );
}

describe("page reader four-state matrix", () => {
  it("shows the skeleton, then the page with its h1 kept and source meta", async () => {
    const requested: string[] = [];
    server.use(
      http.get(PAGES_ENDPOINT, ({ request }) => {
        requested.push(new URL(request.url).pathname);
        return undefined; // record only — the fixture answers
      }),
    );
    mountAt(PAGE_URL);

    await waitFor(() => {
      expect(screen.getByTestId("page-skeleton")).toBeInTheDocument();
    });

    await findRenderedGuide();
    expect(screen.queryByTestId("page-skeleton")).not.toBeInTheDocument();

    // Footer meta names the RECONSTRUCTED source path + short sha.
    expect(screen.getByTestId("page-meta")).toHaveTextContent(
      "docs/guides/local-dev.md · fixture",
    );

    // The nested splat travelled as ONE percent-encoded segment — the
    // spelling the spec blesses (orval interpolates params raw).
    expect(
      requested.some((path) => path.endsWith("/pages/guides%2Flocal-dev.md")),
    ).toBe(true);

    // Breadcrumbs: repo crumb links home; splat segments are static.
    expect(
      screen.getByRole("link", { name: "donaldgifford/docz-site" }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site");
    expect(screen.getByText("local-dev.md")).toBeInTheDocument();
  });

  it("renders the neutral panel on 404", async () => {
    mountAt("/donaldgifford/docz-site/pages/no-such-page.md");
    expect(
      await screen.findByText("Not found — or not visible to you"),
    ).toBeInTheDocument();
  });

  it("redirects to /login on 401 and stashes the destination", async () => {
    server.use(
      http.get(PAGES_ENDPOINT, () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt(PAGE_URL);

    expect(
      await screen.findByRole("link", { name: "Continue with GitHub" }),
    ).toHaveAttribute("href", "/auth/login?provider=github");
    expect(sessionStorage.getItem("docz:auth:return-to")).toBe(PAGE_URL);
  });

  it("treats a 503 as a retryable outage, never a logout", async () => {
    let calls = 0;
    server.use(
      http.get(PAGES_ENDPOINT, () => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json(
            { error: "session unavailable" },
            { status: 503 },
          );
        }
        return undefined; // fall through to the fixture handler
      }),
    );
    mountAt(PAGE_URL);

    expect(await screen.findByText("session unavailable")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();
    expect(sessionStorage.getItem("docz:auth:return-to")).toBeNull();

    await userEvent.click(screen.getByRole("button", { name: "retry" }));
    await findRenderedGuide();
  });
});

describe("route shape", () => {
  it("redirects the empty splat to the repo home", async () => {
    const router = mountAt("/donaldgifford/docz-site/pages");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/donaldgifford/docz-site");
    });
    // The landing page IS the repo home (DESIGN-0004 OQ-2a).
    expect(
      await screen.findByRole("heading", { level: 1, name: "docz-site" }),
    ).toBeInTheDocument();
  });

  it("outranks :type — an additional_docs page renders, not a type 404", async () => {
    mountAt("/donaldgifford/docz-site/pages/README.md");

    // "pages" is not a doc type; if :type had matched first this would
    // be the neutral panel. The reader's meta footer proves the page
    // route won, and the additional_docs member keeps its identity
    // (repo-relative source path, no docs_dir join).
    await screen.findByRole(
      "heading",
      { level: 1, name: "docz-site" },
      { timeout: 10_000 },
    );
    expect(screen.getByTestId("page-meta")).toHaveTextContent("README.md ·");
    expect(
      screen.queryByText("Not found — or not visible to you"),
    ).not.toBeInTheDocument();
  });
});

describe("doc↔page link resolution", () => {
  it("resolves a page's relative link to a doc reader route", async () => {
    // The design index (published at "design", source
    // docs/design/README.md) links its docs by filename — a byPath hit
    // on DESIGN-0001's ingested path.
    mountAt("/donaldgifford/docz-site/pages/design");

    const link = await screen.findByRole(
      "link",
      { name: "0001-docz-site-cross-repo-docz-reader-and-search-ui.md" },
      { timeout: 10_000 },
    );
    expect(link).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/design/DESIGN-0001",
    );
  });

  it("resolves a doc's relative link to a page route", async () => {
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs/:docId", () =>
        HttpResponse.json({
          repo: "donaldgifford/docz-site",
          doc_id: "DESIGN-0001",
          type: "design",
          title: "Synthetic doc",
          status: "Draft",
          author: "Donald Gifford",
          created: "2026-08-01",
          path: "docs/design/0001-synthetic.md",
          git_sha: "fixture-sha-synthetic",
          content_hash: "fixture-hash-synthetic",
          updated_at: "2026-08-01T00:00:00Z",
          raw_md: "# Synthetic doc\n\n[the guide](../guides/local-dev.md)\n",
        }),
      ),
    );
    mountAt("/donaldgifford/docz-site/design/DESIGN-0001");

    const link = await screen.findByRole(
      "link",
      { name: "the guide" },
      { timeout: 10_000 },
    );
    expect(link).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/guides/local-dev.md",
    );
  });

  it("drops the page's own source path but resolves its siblings", async () => {
    server.use(
      http.get(PAGES_ENDPOINT, () =>
        HttpResponse.json({
          repo: "donaldgifford/docz-site",
          path: "guides/local-dev.md",
          title: "Local dev",
          raw_md:
            "# Local dev\n\n[self](local-dev.md)\n\n[the input doc](../input.md)\n",
          git_sha: "syntheticsha",
        }),
      ),
    );
    mountAt(PAGE_URL);

    // The sibling resolves (base docs/guides/local-dev.md → ../input.md
    // → docs/input.md, the input.md page's reconstructed source)…
    const sibling = await screen.findByRole(
      "link",
      { name: "the input doc" },
      { timeout: 10_000 },
    );
    expect(sibling).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/pages/input.md",
    );

    // …while the self-link resolves to the page's OWN source keys and
    // stays the author's untouched relative href.
    const self = screen.getByRole("link", { name: "self" });
    expect(self).toHaveAttribute("href", "local-dev.md");
    expect(self).not.toHaveAttribute("data-xref");
  });
});

describe("non-opted repos", () => {
  it("fires zero pages requests when the api: block is absent", async () => {
    const pagesRequests: string[] = [];
    server.use(
      http.get("*/api/v1/repos/:owner/:name/pages", ({ request }) => {
        pagesRequests.push(request.url);
        return undefined;
      }),
      http.get(PAGES_ENDPOINT, ({ request }) => {
        pagesRequests.push(request.url);
        return undefined;
      }),
    );
    // docz-api's fixture snapshot has no api: block — its repo home
    // builds the full link index without ever asking for pages.
    mountAt("/donaldgifford/docz-api");

    await screen.findByRole(
      "heading",
      { level: 1, name: "docz-api" },
      { timeout: 10_000 },
    );
    expect(pagesRequests).toEqual([]);
  });
});
