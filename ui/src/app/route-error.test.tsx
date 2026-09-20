import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  createMemoryRouter,
  RouterProvider,
  type RouteObject,
} from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "@/app/AppShell";
import { RouteErrorBoundary } from "@/app/route-error";
import {
  BOOM_MESSAGE,
  BOOM_PATH,
  routesWithThrow,
} from "@/test/throwing-routes";

/*
 * Route error boundary (DESIGN-0006 Component 9). Before this, a render
 * throw unmounted the tree to a blank page (INV-0006 F9).
 *
 * The placement is the part worth pinning: the boundary must sit BELOW
 * AppShell so the topbar survives the crash. On the root route it would
 * replace AppShell itself, and a user stranded on a panel with no
 * navigation is the failure this exists to fix.
 */

function Boom(): never {
  throw new Error(BOOM_MESSAGE);
}

function mountAt(path: string, table: RouteObject[] = routesWithThrow()) {
  const router = createMemoryRouter(table, { initialEntries: [path] });
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

beforeEach(() => {
  // React logs caught render errors itself; silence the expected noise
  // so a passing run stays readable.
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("route error boundary", () => {
  it("renders the panel instead of a blank page when a route throws", async () => {
    mountAt(BOOM_PATH);
    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByText(BOOM_MESSAGE)).toBeInTheDocument();
  });

  it("keeps the topbar, so the user is not stranded", async () => {
    mountAt(BOOM_PATH);
    await screen.findByText("Something went wrong");

    const nav = within(screen.getByRole("navigation"));
    expect(nav.getByRole("link", { name: "Directory" })).toBeInTheDocument();
    expect(nav.getByRole("link", { name: "Repos" })).toBeInTheDocument();
  });

  it("offers a link home as the only recovery affordance", async () => {
    mountAt(BOOM_PATH);
    await screen.findByText("Something went wrong");

    // No retry button: re-rendering the same crashed route just throws
    // again, which reads as a control that does nothing (OQ-5a).
    expect(
      screen.queryByRole("button", { name: /retry/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "go home" })).toHaveAttribute(
      "href",
      "/",
    );
  });

  it("navigates away from the crashed route", async () => {
    const router = mountAt(BOOM_PATH);
    await screen.findByText("Something went wrong");

    await userEvent.click(screen.getByRole("link", { name: "go home" }));
    expect(router.state.location.pathname).toBe("/");
    // The destination is a lazy route, so the panel is still mounted for
    // a tick after the location changes — assert on the settled state,
    // not on the instant after the click. (CI caught this as a flake
    // that a fast local machine hid.)
    await waitFor(() => {
      expect(
        screen.queryByText("Something went wrong"),
      ).not.toBeInTheDocument();
    });
  });

  it("does not swallow the error — console.error still sees it", async () => {
    mountAt(BOOM_PATH);
    await screen.findByText("Something went wrong");

    const logged = vi.mocked(console.error).mock.calls;
    expect(
      logged.some(
        (args) =>
          typeof args[0] === "string" &&
          args[0].includes("route error boundary caught"),
      ),
    ).toBe(true);
  });

  it("describes a router error response by status, not as a crash", async () => {
    // isRouteErrorResponse is a separate branch from a thrown Error.
    // It fires for a Response thrown from a LOADER — throwing one from
    // render yields a plain value the router does not wrap, so the
    // loader is the honest way to exercise this path.
    const table: RouteObject[] = [
      {
        path: "/",
        Component: AppShell,
        HydrateFallback: () => null,
        children: [
          {
            ErrorBoundary: RouteErrorBoundary,
            children: [
              {
                path: "gone",
                loader: () => {
                  // Throwing a Response from a loader is react-router's
                  // documented way to produce a route error response —
                  // the only input that reaches the isRouteErrorResponse
                  // branch. Narrowly disabled; the rule is right about
                  // application code.
                  // eslint-disable-next-line @typescript-eslint/only-throw-error -- react-router loader convention
                  throw new Response("nope", {
                    status: 404,
                    statusText: "Not Found",
                  });
                },
                Component: () => null,
              },
            ],
          },
        ],
      },
    ];
    mountAt("/gone", table);
    expect(await screen.findByText("404 Not Found")).toBeInTheDocument();
  });
});

describe("the boundary's placement", () => {
  it("would lose the topbar if it owned the root route", async () => {
    // Not a hypothetical: DESIGN-0006 Component 9 describes putting the
    // errorElement on `path: "/"` and expecting it to render inside
    // AppShell. It does not — a boundary replaces the element of the
    // route that owns it. This pins the reason for the pathless route.
    const table: RouteObject[] = [
      {
        path: "/",
        Component: AppShell,
        ErrorBoundary: RouteErrorBoundary,
        HydrateFallback: () => null,
        children: [{ path: BOOM_PATH.slice(1), Component: Boom }],
      },
    ];
    mountAt(BOOM_PATH, table);

    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();
    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
  });
});
