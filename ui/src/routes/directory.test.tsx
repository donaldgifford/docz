import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { act } from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

import type { SearchHit } from "@/api/__generated__/docz-api.schemas";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";
const SITE_IMPL_TITLE =
  "docz-site MVP: phased build of the reader, directory, and repo pages";
const API_DESIGN_TITLE =
  "docz-api cross-repo docz registry and ingestion service";
const API_CONTRACT_TITLE = "OpenAPI contract for docz-api and the docz-site";

/** A search handler over `total` synthetic docs honoring offset/limit. */
function syntheticSearchHandler(total: number) {
  return http.get("*/api/v1/search", ({ request }) => {
    const url = new URL(request.url);
    const offset = Number.parseInt(url.searchParams.get("offset") ?? "0", 10);
    const limit = Number.parseInt(url.searchParams.get("limit") ?? "20", 10);
    const hits: SearchHit[] = Array.from({ length: total }, (_, i) => ({
      repo: "acme/docs",
      doc_id: `DOC-${String(i).padStart(4, "0")}`,
      type: "guide",
      title: `Synthetic doc ${String(i)}`,
      status: "Draft",
      author: "someone",
      snippet: "",
    })).slice(offset, offset + limit);
    return HttpResponse.json({
      query: url.searchParams.get("q") ?? "",
      estimated_total_hits: total,
      hits,
      facets: {
        repo: { "acme/docs": total },
        type: { guide: total },
        status: { Draft: total },
        author: { someone: total },
      },
    });
  });
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

describe("directory route", () => {
  it("lists every demo doc for the empty query, updated column unset", async () => {
    mountAt("/");

    expect(await screen.findByText(SITE_DESIGN_TITLE)).toBeInTheDocument();
    for (const title of [
      SITE_IMPL_TITLE,
      API_DESIGN_TITLE,
      API_CONTRACT_TITLE,
    ]) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }

    // SearchHit has no updated_at (additive ask) — every row renders "—".
    expect(screen.getAllByText("—")).toHaveLength(4);

    // Rows link straight into the reader.
    expect(
      screen.getByRole("link", { name: new RegExp(API_CONTRACT_TITLE) }),
    ).toHaveAttribute("href", "/donaldgifford/docz-api/design/DESIGN-0002");
  });

  it("binds URL facet params to the query (?type=impl)", async () => {
    mountAt("/?type=impl");

    expect(await screen.findByText(SITE_IMPL_TITLE)).toBeInTheDocument();
    expect(screen.queryByText(SITE_DESIGN_TITLE)).not.toBeInTheDocument();
    expect(screen.queryByText(API_DESIGN_TITLE)).not.toBeInTheDocument();
  });

  it("seeds the search box from the URL", async () => {
    mountAt("/?q=hello");

    const box = await screen.findByRole("searchbox", {
      name: "Search documents",
    });
    expect(box).toHaveValue("hello");
  });

  it("debounces typed queries into the URL", async () => {
    const user = userEvent.setup();
    const router = mountAt("/");
    await screen.findByText(SITE_DESIGN_TITLE);

    const box = screen.getByRole("searchbox", { name: "Search documents" });
    await user.type(box, "registry");

    // Not committed synchronously — the ~200 ms debounce is pending.
    expect(router.state.location.search).toBe("");
    await waitFor(() => {
      expect(router.state.location.search).toBe("?q=registry");
    });
  });

  it("yields the draft to external URL changes (back/forward)", async () => {
    const router = mountAt("/?q=first");
    const box = await screen.findByRole("searchbox", {
      name: "Search documents",
    });
    expect(box).toHaveValue("first");

    await act(async () => {
      await router.navigate("/?q=second");
    });
    await waitFor(() => {
      expect(box).toHaveValue("second");
    });
  });

  it("drives the repo picker, chips, and count line from facets", async () => {
    const user = userEvent.setup();
    mountAt("/");
    await screen.findByText(SITE_DESIGN_TITLE);

    expect(screen.getByTestId("results-count")).toHaveTextContent(
      "showing 4 of 4",
    );

    // Chips are the union of type facet values, plus the all-types reset.
    expect(
      screen.getByRole("button", { name: "all types" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "design" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "impl" })).toBeInTheDocument();

    // Picker menu lists per-repo counts and the cross-repo total.
    await user.click(screen.getByRole("button", { name: /repo:/ }));
    expect(
      screen.getByRole("button", { name: "all repos 4" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "donaldgifford/docz-site 2" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "donaldgifford/docz-api 2" }),
    ).toBeInTheDocument();
  });

  it("chip click filters rows, keeps every chip visible, pushes history", async () => {
    const user = userEvent.setup();
    const router = mountAt("/");
    await screen.findByText(SITE_DESIGN_TITLE);

    await user.click(screen.getByRole("button", { name: "impl" }));
    await waitFor(() => {
      expect(router.state.location.search).toBe("?type=impl");
    });
    await waitFor(() => {
      expect(screen.queryByText(SITE_DESIGN_TITLE)).not.toBeInTheDocument();
    });
    expect(screen.getByText(SITE_IMPL_TITLE)).toBeInTheDocument();
    expect(screen.getByTestId("results-count")).toHaveTextContent(
      "showing 1 of 1",
    );

    // The facet source excludes its own dimension, so unselected type
    // chips stay offered while one is active.
    expect(screen.getByRole("button", { name: "design" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "impl" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    // Filter changes push history — back restores the unfiltered view.
    await act(async () => {
      await router.navigate(-1);
    });
    await waitFor(() => {
      expect(router.state.location.search).toBe("");
    });
    expect(await screen.findByText(SITE_DESIGN_TITLE)).toBeInTheDocument();
  });

  it("scopes to a repo through the picker", async () => {
    const user = userEvent.setup();
    const router = mountAt("/");
    await screen.findByText(SITE_DESIGN_TITLE);

    await user.click(screen.getByRole("button", { name: /repo:/ }));
    await user.click(
      screen.getByRole("button", { name: "donaldgifford/docz-api 2" }),
    );

    await waitFor(() => {
      expect(router.state.location.search).toBe(
        `?repo=${encodeURIComponent("donaldgifford/docz-api")}`,
      );
    });
    await waitFor(() => {
      expect(screen.queryByText(SITE_DESIGN_TITLE)).not.toBeInTheDocument();
    });
    expect(screen.getByText(API_DESIGN_TITLE)).toBeInTheDocument();
  });

  it("clear filters resets to the empty state", async () => {
    const user = userEvent.setup();
    const router = mountAt("/?type=impl&q=phased");
    await screen.findByText(SITE_IMPL_TITLE);

    await user.click(screen.getByRole("button", { name: /clear filters/ }));
    await waitFor(() => {
      expect(router.state.location.search).toBe("");
    });
    expect(await screen.findByText(SITE_DESIGN_TITLE)).toBeInTheDocument();
    expect(
      screen.getByRole("searchbox", { name: "Search documents" }),
    ).toHaveValue("");
  });

  it("load more grows the window through the URL offset", async () => {
    server.use(syntheticSearchHandler(60));
    const user = userEvent.setup();
    const router = mountAt("/");

    await screen.findByText("Synthetic doc 0");
    expect(screen.getAllByRole("listitem")).toHaveLength(25);
    expect(screen.getByTestId("results-count")).toHaveTextContent(
      "showing 25 of 60",
    );

    await user.click(screen.getByRole("button", { name: /load more/ }));
    await waitFor(() => {
      expect(router.state.location.search).toBe("?offset=25");
    });
    await screen.findByText("Synthetic doc 49");
    expect(screen.getAllByRole("listitem")).toHaveLength(50);

    await user.click(screen.getByRole("button", { name: /load more/ }));
    await screen.findByText("Synthetic doc 59");
    expect(screen.getAllByRole("listitem")).toHaveLength(60);
    // Whole set shown — nothing left to load.
    expect(
      screen.queryByRole("button", { name: /load more/ }),
    ).not.toBeInTheDocument();
  });

  it("renders the full window for a deep-linked offset", async () => {
    server.use(syntheticSearchHandler(60));
    mountAt("/?offset=25");

    await screen.findByText("Synthetic doc 49");
    expect(screen.getAllByRole("listitem")).toHaveLength(50);
    expect(screen.getByText("Synthetic doc 0")).toBeInTheDocument();
  });

  it("shows skeleton rows before the first page resolves", async () => {
    mountAt("/");

    await waitFor(() => {
      expect(screen.getByTestId("directory-skeleton")).toBeInTheDocument();
    });
    await screen.findByText(SITE_DESIGN_TITLE);
    expect(screen.queryByTestId("directory-skeleton")).not.toBeInTheDocument();
  });
});
