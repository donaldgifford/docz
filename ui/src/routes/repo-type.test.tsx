import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { server } from "@/test/server";

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

describe("repo type page", () => {
  it("synthesizes the README-style page with the doc table", async () => {
    mountAt("/donaldgifford/docz-site/design");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Designs" }),
    ).toBeInTheDocument();
    // Curated blurb for the standard design type.
    expect(
      screen.getByText(/the shape of a solution before it is built/),
    ).toBeInTheDocument();
    // docz-create hint.
    expect(
      screen.getByText(/docz create design "Your design title"/),
    ).toBeInTheDocument();

    // Table row: ID links to the reader; status, date, filename shown.
    const idLink = await screen.findByRole("link", { name: "DESIGN-0001" });
    expect(idLink).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/design/DESIGN-0001",
    );
    expect(screen.getByRole("columnheader", { name: "Date" })).toBeVisible();
    expect(
      screen.getByText(
        "0001-docz-site-cross-repo-docz-reader-and-search-ui.md",
      ),
    ).toBeInTheDocument();
  });

  it("resolves id_prefix and alias URLs to the canonical type", async () => {
    mountAt("/donaldgifford/docz-site/INV");

    // "INV" is the id_prefix of type "investigation" (alias "inv").
    expect(
      await screen.findByRole("heading", { level: 1, name: "Investigations" }),
    ).toBeInTheDocument();
  });

  it("shows the docz-create empty state for a type with zero docs", async () => {
    mountAt("/donaldgifford/docz-api/rfc");

    expect(
      await screen.findByRole("heading", { level: 1, name: "RFCs" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/No rfc documents yet/)).toBeInTheDocument();
    expect(screen.getByText(/scaffolds the first one/)).toBeInTheDocument();
  });

  it("renders the neutral panel for an unknown type", async () => {
    mountAt("/donaldgifford/docz-site/nonsense");

    expect(
      await screen.findByText("Not found — or not visible to you"),
    ).toBeInTheDocument();
  });

  it("renders an inline error whose retry recovers", async () => {
    let failing = true;
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs", () =>
        failing
          ? HttpResponse.json({ error: "boom" }, { status: 500 })
          : undefined,
      ),
    );
    mountAt("/donaldgifford/docz-site/design");

    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();

    failing = false;
    await userEvent.click(screen.getByRole("button", { name: "retry" }));
    expect(
      await screen.findByRole("heading", { level: 1, name: "Designs" }),
    ).toBeInTheDocument();
  });

  it("renders the bare session panel on 401", async () => {
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs", () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt("/donaldgifford/docz-site/design");

    expect(await screen.findByText("Session required")).toBeInTheDocument();
  });
});
