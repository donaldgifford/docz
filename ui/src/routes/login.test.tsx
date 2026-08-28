import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

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

describe("login page", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

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

  it("remembers the clicked provider", async () => {
    mountAt("/login");
    const user = userEvent.setup();
    // jsdom can't navigate — swallow the anchor's default action; the
    // React onClick has already run by the time it bubbles here.
    const stopNavigation = (event: Event) => {
      event.preventDefault();
    };
    document.addEventListener("click", stopNavigation);

    await user.click(
      await screen.findByRole("link", { name: "Continue with GitHub" }),
    );
    document.removeEventListener("click", stopNavigation);

    expect(localStorage.getItem("docz:auth:last-provider")).toBe("github");
  });

  it("swaps to the auth-disabled panel in none-mode", async () => {
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

    expect(await screen.findByTestId("login-auth-disabled")).toHaveTextContent(
      "Authentication is disabled",
    );
    expect(
      screen.getByRole("link", { name: "Browse the docs" }),
    ).toHaveAttribute("href", "/");
    // The /auth routes aren't mounted upstream in none-mode — the page
    // must offer zero anchors at them.
    expect(screen.queryAllByRole("link", { name: /Continue with/ })).toEqual(
      [],
    );
    expect(
      document.querySelector('a[href^="/auth/login"]'),
    ).not.toBeInTheDocument();
  });

  it("keeps the provider buttons through a failed or pending probe", async () => {
    // 503: the session state is unknown, not anonymous — never swap.
    server.use(
      http.get("*/api/v1/auth/session", () =>
        HttpResponse.json({ error: "session unavailable" }, { status: 503 }),
      ),
    );
    mountAt("/login");

    expect(
      await screen.findByRole("link", { name: "Continue with GitHub" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByTestId("session-pending")).not.toBeInTheDocument();
    });
    expect(screen.queryByTestId("login-auth-disabled")).not.toBeInTheDocument();
  });

  it("promotes the last-used provider to the primary slot", async () => {
    vi.stubEnv("VITE_AUTH_PROVIDERS", "github,okta");
    localStorage.setItem("docz:auth:last-provider", "okta");
    mountAt("/login");

    const links = await screen.findAllByRole("link", { name: /Continue/ });
    expect(links.map((link) => link.getAttribute("data-testid"))).toEqual([
      "login-okta",
      "login-github",
    ]);
    // accessible-name computation collapses the joining whitespace
    expect(links[0]).toHaveAccessibleName(/Continue with Okta ?· last used/);
  });
});
