import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";
import { DEMO_DOCS } from "@/mocks/fixtures";
import { server } from "@/test/server";

const DOC_URL = "/donaldgifford/docz-site/design/DESIGN-0001";
const DOC_ENDPOINT = "*/api/v1/repos/:owner/:name/types/:type/docs/:docId";

function mountAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

async function findRenderedDesign0001() {
  return await screen.findByRole(
    "heading",
    { level: 1, name: "docz-site: cross-repo docz reader and search UI" },
    { timeout: 10_000 },
  );
}

describe("reader four-state matrix", () => {
  it("shows the skeleton while loading, then the document", async () => {
    mountAt(DOC_URL);

    await waitFor(() => {
      expect(screen.getByTestId("doc-skeleton")).toBeInTheDocument();
    });

    await findRenderedDesign0001();
    expect(screen.queryByTestId("doc-skeleton")).not.toBeInTheDocument();
    // id line derived from doc_id
    expect(screen.getByText("DESIGN / 0001")).toBeInTheDocument();
    // breadcrumb current segment is the file name from `path`
    expect(
      screen.getByText(
        "0001-docz-site-cross-repo-docz-reader-and-search-ui.md",
      ),
    ).toBeInTheDocument();
  });

  it("renders the neutral panel on 404", async () => {
    mountAt("/donaldgifford/docz-site/design/DESIGN-9999");
    expect(
      await screen.findByText("Not found — or not visible to you"),
    ).toBeInTheDocument();
  });

  it("renders the bare session panel on 401", async () => {
    server.use(
      http.get(DOC_ENDPOINT, () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt(DOC_URL);

    expect(await screen.findByText("Session required")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Sign in with GitHub" }),
    ).toHaveAttribute("href", "/auth/login?provider=github");
  });

  it("renders an inline error with a working retry", async () => {
    let calls = 0;
    server.use(
      http.get(DOC_ENDPOINT, () => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json(
            { error: "meilisearch is on fire" },
            { status: 500 },
          );
        }
        return undefined; // fall through to the fixture handler
      }),
    );
    mountAt(DOC_URL);

    expect(
      await screen.findByText("meilisearch is on fire"),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "retry" }));
    await findRenderedDesign0001();
  });
});

describe("metadata card omission", () => {
  it("omits empty-string fields and falls back in the header", async () => {
    const base = DEMO_DOCS.find((doc) => doc.doc_id === "DESIGN-0001");
    server.use(
      http.get(DOC_ENDPOINT, () =>
        HttpResponse.json({
          ...base,
          status: "",
          author: "",
          created: "",
          git_sha: "",
        }),
      ),
    );
    mountAt(DOC_URL);
    await findRenderedDesign0001();

    // header meta: no pill, author falls back
    expect(screen.getByText("unassigned")).toBeInTheDocument();
    // rail metadata rows for ""-valued fields are gone entirely
    expect(screen.queryByText("Status")).not.toBeInTheDocument();
    expect(screen.queryByText("Author")).not.toBeInTheDocument();
    expect(screen.queryByText("Created")).not.toBeInTheDocument();
    expect(screen.queryByText("Commit")).not.toBeInTheDocument();
    // non-empty fields stay
    expect(screen.getByText("Updated")).toBeInTheDocument();
    expect(screen.getByText("all fields · json →")).toHaveAttribute(
      "href",
      "/api/v1/repos/donaldgifford/docz-site/types/design/docs/DESIGN-0001",
    );
  });
});

describe("lifecycle rail positioning", () => {
  // The docz-api design doc is ~1200 lines; give the pipeline headroom.
  it(
    "marks stops done/current/pending around the doc status",
    { timeout: 20_000 },
    async () => {
      // docz-api DESIGN-0001 is Approved: Draft, In Review done; Approved
      // current; Implemented, Abandoned pending.
      mountAt("/donaldgifford/docz-api/design/DESIGN-0001");
      await screen.findByRole(
        "heading",
        { level: 1, name: /docz-api cross-repo docz registry/ },
        { timeout: 10_000 },
      );

      const rail = await screen.findByTestId("lifecycle-rail");
      const states = [...rail.querySelectorAll("[data-lifecycle-state]")].map(
        (el) => [el.textContent, el.getAttribute("data-lifecycle-state")],
      );
      expect(states).toEqual([
        ["Draft", "done"],
        ["In Review", "done"],
        ["Approved", "current"],
        ["Implemented", "pending"],
        ["Abandoned", "pending"],
      ]);
    },
  );
});

describe("portal sibling navigation", () => {
  it("swaps documents through the repo nav without a reload", async () => {
    mountAt(DOC_URL);
    await findRenderedDesign0001();

    // The repo nav frames the reader and lists sibling docs.
    const navLink = await screen.findByRole("link", {
      name: /IMPL-0001 · docz-site MVP/,
    });
    await userEvent.click(navLink);

    expect(
      await screen.findByRole(
        "heading",
        { level: 1, name: /docz-site MVP: phased build/ },
        { timeout: 10_000 },
      ),
    ).toBeInTheDocument();
    // The previous doc is gone from the article; the nav still lists it.
    expect(navLink).toHaveAttribute("aria-current", "page");
  });
});

describe("ToC anchor navigation", () => {
  it("links every collected heading to a real anchor in the article", async () => {
    mountAt(DOC_URL);
    await findRenderedDesign0001();

    const tocLists = screen.getAllByText("On this page");
    expect(tocLists.length).toBeGreaterThan(0);

    const anchors = document.querySelectorAll('a[href^="#"]');
    expect(anchors.length).toBeGreaterThan(5);
    for (const anchor of anchors) {
      const id = anchor.getAttribute("href")?.slice(1) ?? "";
      expect(
        document.getElementById(id),
        `#${id} has no target`,
      ).not.toBeNull();
    }
  });
});
