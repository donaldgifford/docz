import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { routes } from "@/app/router";

/*
 * The topbar's appearance controls (IMPL-0030 Phase 3): the deployment's
 * default theme, a reader's pick stored under docz:theme, and the
 * light/dark toggle on a theme with modes.
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
  root.setAttribute("data-theme", "paper");
});

describe("topbar appearance", () => {
  it("starts on the deployment's theme", async () => {
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    mountAt("/repos");
    const select = await screen.findByLabelText("Theme");
    expect((select as HTMLSelectElement).value).toBe("classic");
    expect(root.getAttribute("data-theme")).toBe("classic");
    expect(screen.queryByRole("button", { name: /Switch to/ })).toBeNull();
  });

  it("stores the reader's pick and offers the toggle on paper", async () => {
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    const user = userEvent.setup();
    mountAt("/repos");
    await user.selectOptions(await screen.findByLabelText("Theme"), "paper");
    expect(localStorage.getItem("docz:theme")).toBe("paper");
    expect(root.getAttribute("data-theme")).toBe("paper");
    await user.click(screen.getByRole("button", { name: /Switch to/ }));
    expect(localStorage.getItem("docz:mode")).not.toBeNull();
    expect(root.hasAttribute("data-mode")).toBe(true);
  });
});
