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

describe("session menu", () => {
  it("shows the GitHub login and signs out through /login", async () => {
    let signedIn = true;
    server.use(
      http.get("*/api/v1/auth/session", () =>
        signedIn
          ? HttpResponse.json({
              provider: "github",
              subject: "1138",
              login: "donaldgifford",
            })
          : HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
      http.post("*/api/v1/auth/logout", () => {
        signedIn = false;
        return HttpResponse.json({ status: "signed out" });
      }),
    );
    const router = mountAt("/repos");
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: "Account: donaldgifford" }),
    );
    expect(screen.getByText("via github")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Sign out" }));

    expect(
      await screen.findByRole("link", { name: "Continue with GitHub" }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
    // The cleared cache refetches the now-dead session — the topbar
    // flips to the signed-out affordance, proving nothing fetched
    // under the old session survived.
    expect(await screen.findByTestId("topbar-sign-in")).toBeInTheDocument();
  });

  it("falls back to email for OIDC identities", async () => {
    server.use(
      http.get("*/api/v1/auth/session", () =>
        HttpResponse.json({
          provider: "okta",
          subject: "u-1138",
          email: "dev@example.com",
        }),
      ),
    );
    mountAt("/repos");

    expect(
      await screen.findByRole("button", { name: "Account: dev@example.com" }),
    ).toBeInTheDocument();
  });

  it("offers Sign in when there is no session", async () => {
    server.use(
      http.get("*/api/v1/auth/session", () =>
        HttpResponse.json({ error: "session required" }, { status: 401 }),
      ),
    );
    mountAt("/repos");

    expect(await screen.findByTestId("topbar-sign-in")).toHaveAttribute(
      "href",
      "/login",
    );
  });

  it("closes on Escape and hands focus back to the trigger", async () => {
    // fixture session: donaldgifford
    mountAt("/repos");
    const user = userEvent.setup();

    const trigger = await screen.findByRole("button", {
      name: "Account: donaldgifford",
    });
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await user.keyboard("{Escape}");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });
});
