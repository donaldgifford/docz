import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { routes } from "@/app/router";
import { DEMO_DOCS } from "@/mocks/fixtures";
import { failDocFragments, server } from "@/test/server";

/*
 * The reader's document pane (DESIGN-0026 §7, IMPL-0032 Phase 4): a
 * document is its docz-html fragment from docz-api, in the deployment's
 * document theme, with pkg/theme's runtime mounted inside it, the tab in
 * ?tab=, and server-resolved site links routed in-app. The fixtures'
 * fragments are docz-api's own renderer's output (fragments.json).
 */

// mermaid never settles here: the pane's job is to hand the source to
// MermaidBlock, whose loading state keeps it visible as text.
const pending = vi.hoisted(() => () => new Promise<never>(() => undefined));
vi.mock("mermaid", () => ({
  default: { initialize: vi.fn(), render: vi.fn(pending) },
}));

const RFC = "/donaldgifford/docz-api/rfc/RFC-0001";

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

async function findPane(): Promise<HTMLElement> {
  return await screen.findByTestId("doc-pane", {}, { timeout: 10_000 });
}

function page(pane: HTMLElement): HTMLElement {
  const el = pane.querySelector<HTMLElement>(".docz-page");
  if (el === null) {
    throw new Error("the pane holds no .docz-page");
  }
  return el;
}

afterEach(() => {
  delete window.__DOCZ_CONFIG__;
});

describe("the document pane", () => {
  it("shows the fragment in the deployment's document theme", async () => {
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    mountAt(RFC);
    const pane = await findPane();
    const doc = page(pane);
    expect(doc).toHaveAttribute("data-doc", "RFC-0001");
    expect(doc).toHaveAttribute("data-theme", "classic");
    // The runtime mounted: it marks the page and selects a tab.
    expect(doc).toHaveClass("js");
    expect(
      within(doc).getByRole("tab", { selected: true }),
    ).toBeInTheDocument();
    // The fragment carries the title; the site's header and rail do not
    // repeat it, and no fallback note shows.
    expect(screen.queryByTestId("doc-fallback-note")).toBeNull();
    expect(screen.queryByText("RFC / 0001")).toBeNull();
    // The shell above the pane stays.
    expect(screen.getByText("Metadata")).toBeInTheDocument();
  });

  it("keeps the tab in ?tab= and opens the one the URL names", async () => {
    const user = userEvent.setup();
    const router = mountAt(`${RFC}?tab=decision`);
    const doc = page(await findPane());
    expect(within(doc).getByRole("tab", { selected: true })).toHaveAttribute(
      "data-tab",
      "decision",
    );

    await user.click(within(doc).getByRole("tab", { name: /References/ }));
    expect(router.state.location.search).toBe("?tab=references");
    expect(router.state.location.hash).toBe("");
    expect(doc.querySelector("#p-references")).toBeVisible();
  });

  it("routes a server-resolved site link in-app", async () => {
    const user = userEvent.setup();
    const router = mountAt(`${RFC}?tab=references`);
    const doc = page(await findPane());
    const link = doc.querySelector<HTMLAnchorElement>(
      'a[href="/donaldgifford/docz-api/design/DESIGN-0002"]',
    );
    if (link === null) {
      throw new Error("the fixture lost its DESIGN-0002 link");
    }
    await user.click(link);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/donaldgifford/docz-api/design/DESIGN-0002",
      );
    });
  });

  it("draws a pre.mermaid with the site's mermaid", async () => {
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs/:docId/html", () =>
        HttpResponse.json({
          doc_id: "RFC-0001",
          html: '<div class="docz-page" data-theme="paper" data-doc="RFC-0001"><div class="wrap"><pre class="mermaid">flowchart TD\n  A --&gt; B</pre></div></div>',
          hash: "sha256:x",
          inferred: false,
          problems: [],
        }),
      ),
    );
    mountAt(RFC);
    const doc = page(await findPane());
    await waitFor(() => {
      expect(doc.querySelector("pre.mermaid")).toBeNull();
    });
    // MermaidBlock's loading state keeps the source, as text.
    expect(
      within(doc).getByRole("region", { name: "mermaid diagram source" }),
    ).toHaveTextContent("A --> B");
  });

  it("keeps View in Confluence above the pane", async () => {
    const base = DEMO_DOCS.find((doc) => doc.doc_id === "RFC-0001");
    const url = "https://example.atlassian.net/wiki/spaces/DOCZ/pages/65861";
    server.use(
      http.get("*/api/v1/repos/:owner/:name/types/:type/docs/:docId", () =>
        HttpResponse.json({ ...base, confluence_url: url }),
      ),
    );
    mountAt(RFC);
    await findPane();
    expect(
      screen.getByRole("link", { name: "View in Confluence" }),
    ).toHaveAttribute("href", url);
  });

  it("falls back to the site's render with a note when the fragment fails", async () => {
    failDocFragments();
    mountAt(RFC);
    expect(
      await screen.findByTestId("doc-fallback-note", {}, { timeout: 10_000 }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("doc-pane")).toBeNull();
    expect(screen.getByText("RFC / 0001")).toBeInTheDocument();
  });
});
