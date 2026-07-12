import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { routes } from "@/app/router";

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

describe("login page", () => {
  it("renders the provider buttons as real /auth/login anchors", async () => {
    mountAt("/login");

    const github = await screen.findByRole("link", {
      name: "Continue with GitHub",
    });
    // A NATIVE anchor (full document navigation through the proxy),
    // never a router Link — the 302 must reach the browser.
    expect(github).toHaveAttribute("href", "/auth/login?provider=github");
    expect(
      screen.getByRole("heading", { level: 1, name: "Sign in" }),
    ).toBeInTheDocument();
  });
});
