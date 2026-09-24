import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { routes } from "@/app/router";
import { expectNoAxeViolations } from "@/test/axe";
import { server } from "@/test/server";
import { BOOM_PATH, routesWithThrow } from "@/test/throwing-routes";

// mermaid can't render in jsdom (no SVG measurement); the rejecting
// mock pins the specimen page's diagrams to MermaidBlock's source
// fallback, which is the markup this sweep covers. The real render is
// swept in e2e/a11y.spec.ts.
//
// `default.initialize` and `default.render` are the ENTIRE surface
// MermaidBlock touches, in v12 as in v11 — `mermaidInitConfig()` builds
// the config without a mermaid instance, so nothing here needs to grow
// when that config does. Add a member only when the module starts using
// one, or this mock quietly diverges from what it stands in for.
vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(() => Promise.reject(new Error("jsdom"))),
  },
}));

/*
 * Phase 4 accessibility sweep: every core view mounts against the MSW
 * fixtures, waits for real content, and must produce zero
 * serious/critical axe violations (color-contrast excluded — jsdom
 * can't compute it; see src/test/axe.ts for where contrast IS checked).
 */

const AXE_TIMEOUT = 20_000;

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

describe("axe: core views", () => {
  it(
    "directory with mixed doc/page hits",
    { timeout: AXE_TIMEOUT },
    async () => {
      mountAt("/");
      await screen.findByTestId("results-count", undefined, {
        timeout: 10_000,
      });
      // The fixture org publishes pages — wait for a page row so the
      // sweep covers the mixed-source list (IMPL-0005 Phase 3).
      await screen.findByRole(
        "link",
        { name: /Local development against a real docz-api/ },
        { timeout: 10_000 },
      );
      await expectNoAxeViolations();
    },
  );

  it("repos index", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/repos");
    await screen.findByRole(
      "heading",
      { name: "Repositories" },
      { timeout: 10_000 },
    );
    await screen.findByText("donaldgifford/docz-site");
    await expectNoAxeViolations();
  });

  it("topbar with nav pins", { timeout: AXE_TIMEOUT }, async () => {
    window.__DOCZ_CONFIG__ = {
      nav: [{ label: "RFCs", href: "/donaldgifford/docz-api/rfc" }],
    };
    try {
      mountAt("/repos");
      await screen.findByRole("link", { name: "RFCs" }, { timeout: 10_000 });
      await expectNoAxeViolations();
    } finally {
      delete window.__DOCZ_CONFIG__;
    }
  });

  it("repo home with an index.md", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/donaldgifford/docz-api");
    await screen.findByRole(
      "heading",
      { level: 1, name: /docz-api/ },
      { timeout: 10_000 },
    );
    await expectNoAxeViolations();
  });

  it("repo home, generated fallback", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/donaldgifford/docz-site");
    await screen.findByText(/No/, { exact: false }, { timeout: 10_000 });
    await screen.findByText("configured in docz.yaml.", { exact: false });
    await expectNoAxeViolations();
  });

  it("repo changelog", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/donaldgifford/docz-site/changelog");
    await screen.findByRole(
      "heading",
      { level: 1, name: "Changelog" },
      { timeout: 10_000 },
    );
    await expectNoAxeViolations();
  });

  it("page reader with the pages tree", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/donaldgifford/docz-site/pages/guides/local-dev.md");
    await screen.findByRole(
      "heading",
      { level: 1, name: "Local development against a real docz-api" },
      { timeout: 10_000 },
    );
    // RepoNav's pages tree, active branch auto-expanded — wait for a
    // tree link so the sweep covers it (IMPL-0005 Phase 3).
    await screen.findAllByRole(
      "link",
      { name: "Design Documents" },
      { timeout: 10_000 },
    );
    await expectNoAxeViolations();
  });

  it("markdown specimen page", { timeout: AXE_TIMEOUT }, async () => {
    // docs/guides/markdown-specimen.md: every construct the pipeline
    // renders, on one page — headings, alerts, code chrome, tables,
    // footnotes, raw HTML, task lists — so a regression in any of them
    // trips the sweep here rather than on a real document.
    const { container } = mountAt(
      "/donaldgifford/docz-site/pages/guides/markdown-specimen.md",
    );
    await screen.findByRole(
      "heading",
      { level: 1, name: "Markdown rendering specimen" },
      { timeout: 10_000 },
    );
    // Every mermaid fence must settle on the fallback before the sweep.
    await waitFor(() => {
      expect(
        container.querySelectorAll('[data-mermaid-fallback="failed"]'),
      ).toHaveLength(3);
    });
    await expectNoAxeViolations();
  });

  it("type listing page", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/donaldgifford/docz-site/design");
    await screen.findByRole("table", undefined, { timeout: 10_000 });
    await expectNoAxeViolations();
  });

  it("doc reader", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/donaldgifford/docz-site/design/DESIGN-0001");
    await screen.findByRole(
      "heading",
      { level: 1, name: /docz-site/ },
      { timeout: 10_000 },
    );
    await expectNoAxeViolations();
  });

  it("login page", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/login");
    await screen.findByRole(
      "link",
      { name: "Continue with GitHub" },
      { timeout: 10_000 },
    );
    await expectNoAxeViolations();
  });

  it("login page, none-mode panel", { timeout: AXE_TIMEOUT }, async () => {
    server.use(
      http.get("*/api/v1/auth/session", () =>
        HttpResponse.json({
          provider: "none",
          subject: "anonymous",
          login: "anonymous",
        }),
      ),
    );
    mountAt("/login");
    await screen.findByTestId("login-auth-disabled", undefined, {
      timeout: 10_000,
    });
    await expectNoAxeViolations();
  });

  it("session menu open", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/repos");
    await userEvent.click(
      await screen.findByRole(
        "button",
        { name: "Account: donaldgifford" },
        { timeout: 10_000 },
      ),
    );
    await screen.findByRole("button", { name: "Sign out" });
    await expectNoAxeViolations();
  });

  it("command palette open", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/");
    await screen.findByTestId("results-count", undefined, { timeout: 10_000 });
    await userEvent.keyboard("/");
    await screen.findByRole("dialog", undefined, { timeout: 10_000 });
    await expectNoAxeViolations();
  });

  // The error boundary is the one UI surface DESIGN-0006 adds, so it
  // owes the same sweep as every other view: a panel with no heading or
  // an unfocusable recovery link would fail here (Component 9).
  it("route error boundary", { timeout: AXE_TIMEOUT }, async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    try {
      const router = createMemoryRouter(routesWithThrow(), {
        initialEntries: [BOOM_PATH],
      });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      render(
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>,
      );
      await screen.findByText("Something went wrong");
      await expectNoAxeViolations();
    } finally {
      consoleError.mockRestore();
    }
  });
});
