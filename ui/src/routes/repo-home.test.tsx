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

describe("repo home", () => {
  it("renders index.md through the reader pipeline with a ToC", async () => {
    mountAt("/donaldgifford/docz-api");

    // The index.md h1 is kept — it IS the page title.
    expect(
      await screen.findByRole("heading", { level: 1, name: "docz-api" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 2, name: "What lives here" }),
    ).toBeInTheDocument();

    // ToC in the right rail links to the rendered sections.
    const tocLink = screen.getByRole("link", { name: "Start here" });
    expect(tocLink).toHaveAttribute("href", "#start-here");

    // The shared repo nav frames the page (drawer + desktop copies).
    expect(
      screen.getAllByRole("navigation", {
        name: "donaldgifford/docz-api navigation",
      }).length,
    ).toBeGreaterThan(0);
  });

  it("falls back to the generated home when index.md is absent", async () => {
    mountAt("/donaldgifford/docz-site");

    expect(
      await screen.findByRole("heading", { level: 1, name: "docz-site" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/No/)).toBeInTheDocument();
    expect(screen.getByText(/configured in docz.yaml/)).toBeInTheDocument();

    // One section per configured type, linking to the type pages.
    expect(
      await screen.findByRole("heading", { name: /Designs/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /Implementation Plans/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "browse design/ →" }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site/design");
  });

  it("renders the neutral panel when the repo itself is missing", async () => {
    server.use(
      http.get("*/api/v1/repos/:owner/:name", () =>
        HttpResponse.json({ error: "not found" }, { status: 404 }),
      ),
      http.get("*/api/v1/repos/:owner/:name/index", () =>
        HttpResponse.json({ error: "not found" }, { status: 404 }),
      ),
    );
    mountAt("/nobody/nothing");

    expect(
      await screen.findByText("Not found — or not visible to you"),
    ).toBeInTheDocument();
  });

  it("renders an inline error whose retry recovers", async () => {
    let failing = true;
    server.use(
      http.get("*/api/v1/repos/:owner/:name", () =>
        failing
          ? HttpResponse.json({ error: "boom" }, { status: 500 })
          : undefined,
      ),
    );
    mountAt("/donaldgifford/docz-api");

    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();

    failing = false;
    await userEvent.click(screen.getByRole("button", { name: "retry" }));
    expect(
      await screen.findByRole("heading", { level: 1, name: "docz-api" }),
    ).toBeInTheDocument();
  });

  it("renders the bare session panel on 401", async () => {
    server.use(
      http.get("*/api/v1/repos/:owner/:name", () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
      http.get("*/api/v1/repos/:owner/:name/index", () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt("/donaldgifford/docz-api");

    expect(await screen.findByText("Session required")).toBeInTheDocument();
  });
});
