import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { act } from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";
const SITE_IMPL_TITLE =
  "docz-site MVP: phased build of the reader, directory, and repo pages";
const API_DESIGN_TITLE =
  "docz-api cross-repo docz registry and ingestion service";
const API_CONTRACT_TITLE = "OpenAPI contract for docz-api and the docz-site";

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

  it("shows skeleton rows before the first page resolves", async () => {
    mountAt("/");

    await waitFor(() => {
      expect(screen.getByTestId("directory-skeleton")).toBeInTheDocument();
    });
    await screen.findByText(SITE_DESIGN_TITLE);
    expect(screen.queryByTestId("directory-skeleton")).not.toBeInTheDocument();
  });
});
