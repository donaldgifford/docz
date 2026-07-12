import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, it } from "vitest";

import { routes } from "@/app/router";
import { expectNoAxeViolations } from "@/test/axe";

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
  it("directory", { timeout: AXE_TIMEOUT }, async () => {
    mountAt("/");
    await screen.findByTestId("results-count", undefined, { timeout: 10_000 });
    await expectNoAxeViolations();
  });

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
});
