import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
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

  it("redirects to /login on 401 and stashes the destination", async () => {
    server.use(
      http.get(DOC_ENDPOINT, () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt(DOC_URL);

    expect(
      await screen.findByRole("link", { name: "Continue with GitHub" }),
    ).toHaveAttribute("href", "/auth/login?provider=github");
    expect(sessionStorage.getItem("docz:auth:return-to")).toBe(DOC_URL);
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

describe("metadata table omission", () => {
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
    // table rows for ""-valued fields are gone entirely
    expect(screen.queryByText("Created")).not.toBeInTheDocument();
    expect(screen.queryByText("Commit")).not.toBeInTheDocument();
    // fields the DTO always carries stay
    const table = within(
      screen.getByRole("table", { name: "Document metadata" }),
    );
    expect(
      table.getByRole("rowheader", { name: "Source" }),
    ).toBeInTheDocument();
    expect(
      table.getByRole("link", { name: "donaldgifford/docz-site" }),
    ).toHaveAttribute("href", "/donaldgifford/docz-site");
    expect(screen.getByRole("link", { name: "json" })).toHaveAttribute(
      "href",
      "/api/v1/repos/donaldgifford/docz-site/types/design/docs/DESIGN-0001",
    );
  });

  it("renders the populated table with the format switch", async () => {
    mountAt(DOC_URL);
    await findRenderedDesign0001();

    expect(screen.getByRole("rowheader", { name: "Type" })).toBeInTheDocument();
    expect(screen.getByText("design · DESIGN-0001")).toBeInTheDocument();
    expect(
      screen.getByText(
        "docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md",
        { selector: "td *" },
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("fixture-sha-design-0001".slice(0, 7)),
    ).toBeInTheDocument();

    const group = screen.getByRole("group", { name: "Document format" });
    const html = within(group).getByRole("button", { name: "html" });
    expect(html).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(within(group).getByRole("button", { name: "md" }));
    expect(
      screen.getByRole("region", { name: "raw markdown" }),
    ).toBeInTheDocument();
  });
});

describe("lifecycle disclosure", () => {
  // The docz-api design doc is ~1200 lines; give the pipeline headroom.
  it(
    "stays closed by default and marks stops around the doc status",
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

      const disclosure = await screen.findByTestId("lifecycle-disclosure");
      expect(disclosure).not.toHaveAttribute("open");
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

describe("xref linking in the reader", () => {
  it("links sibling doc ids mentioned in the body", async () => {
    const base = DEMO_DOCS.find((doc) => doc.doc_id === "DESIGN-0001");
    server.use(
      http.get(DOC_ENDPOINT, () =>
        HttpResponse.json({
          ...base,
          raw_md: "# T\n\nSee IMPL-0001 for the build plan.",
          content_hash: "xref-test-hash",
        }),
      ),
    );
    mountAt(DOC_URL);

    const xref = await screen.findByRole(
      "link",
      { name: "IMPL-0001" },
      { timeout: 10_000 },
    );
    expect(xref).toHaveAttribute(
      "href",
      "/donaldgifford/docz-site/impl/IMPL-0001",
    );
    expect(xref).toHaveAttribute("data-xref");
  });
});

describe("portal sibling navigation", () => {
  it("swaps documents through the repo nav without a reload", async () => {
    mountAt(DOC_URL);
    await findRenderedDesign0001();

    // The repo nav frames the reader; other types' drawers start
    // closed, so peek into impl first (the nav renders twice — narrow
    // drawer + desktop rail; either works).
    const [implToggle] = await screen.findAllByRole("button", {
      name: "impl documents",
    });
    if (implToggle === undefined) {
      throw new Error("impl drawer toggle not found");
    }
    await userEvent.click(implToggle);
    const [navLink] = await screen.findAllByRole("link", {
      name: /IMPL-0001 · docz-site MVP/,
    });
    if (navLink === undefined) {
      throw new Error("sibling nav link not found");
    }
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
