import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";
const SITE_IMPL_TITLE =
  "docz-site MVP: phased build of the reader, directory, and repo pages";
const API_DESIGN_TITLE =
  "docz-api cross-repo docz registry and ingestion service";

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

function palette() {
  return within(screen.getByTestId("command-palette"));
}

describe("command palette", () => {
  it("opens with ⌘K on any route and closes with Esc", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument();
    await user.keyboard("{Meta>}k{/Meta}");
    expect(screen.getByTestId("command-palette")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument();
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
    expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument();
    expect(box).toHaveValue("/");

    // From the page body it opens the palette.
    await user.click(document.body);
    await user.keyboard("/");
    expect(screen.getByTestId("command-palette")).toBeInTheDocument();
  });

  it("groups results by repo and previews the highlighted hit", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    const dialog = palette();

    // Empty query lists everything, grouped per repo.
    expect(
      await dialog.findByText("donaldgifford/docz-site — 2 matches"),
    ).toBeInTheDocument();
    expect(
      dialog.getByText("donaldgifford/docz-api — 2 matches"),
    ).toBeInTheDocument();

    // The first hit is highlighted and previewed from hit data alone.
    const preview = screen.getByTestId("palette-preview");
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
    const dialog = palette();
    await dialog.findByText("donaldgifford/docz-site — 2 matches");

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
    const dialog = palette();
    await dialog.findByText("donaldgifford/docz-site — 2 matches");

    // Repo pill (short name) scopes to that repo.
    await user.click(dialog.getByRole("button", { name: "docz-api" }));
    await waitFor(() => {
      expect(
        dialog.queryByText(/donaldgifford\/docz-site —/),
      ).not.toBeInTheDocument();
    });
    expect(
      dialog.getByText("donaldgifford/docz-api — 2 matches"),
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

  it("opens from the topbar search affordance", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await screen.findByText("docz");

    await user.click(
      screen.getByRole("button", { name: /Search docs, rfcs, authors/ }),
    );
    expect(screen.getByTestId("command-palette")).toBeInTheDocument();
  });

  it("navigates to the reader on Enter and closes", async () => {
    const user = userEvent.setup();
    const router = mountAt("/repos");
    await screen.findByText("docz");

    await user.keyboard("{Meta>}k{/Meta}");
    await palette().findByText("donaldgifford/docz-site — 2 matches");

    // ↓ to the second hit, then Enter.
    await user.keyboard("{ArrowDown}{Enter}");
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-site/impl/IMPL-0001",
      );
    });
    expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument();
  });
});
