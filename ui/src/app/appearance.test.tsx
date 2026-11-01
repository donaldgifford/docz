import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { routes } from "@/app/router";

/*
 * The topbar's appearance (DESIGN-0026 §2): the shell has one look with a
 * light/dark toggle, no theme selector, and no data-theme on <html>.
 */

const root = document.documentElement;

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
}

afterEach(() => {
  delete window.__DOCZ_CONFIG__;
  root.removeAttribute("data-mode");
  root.removeAttribute("data-theme");
});

describe("topbar appearance", () => {
  it("offers the toggle and no theme selector, whatever DOCZ_THEME says", async () => {
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    mountAt("/repos");
    expect(
      await screen.findByRole("button", { name: /Switch to/ }),
    ).toBeVisible();
    expect(screen.queryByLabelText("Theme")).toBeNull();
    expect(root.hasAttribute("data-theme")).toBe(false);
  });

  it("stores the toggled mode on <html>", async () => {
    const user = userEvent.setup();
    mountAt("/repos");
    await user.click(await screen.findByRole("button", { name: /Switch to/ }));
    expect(localStorage.getItem("docz:mode")).not.toBeNull();
    expect(root.hasAttribute("data-mode")).toBe(true);
    expect(localStorage.getItem("docz:theme")).toBeNull();
  });
});
