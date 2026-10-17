import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { routes } from "@/app/router";
import { describeScope } from "@/routes/consent";
import { server } from "@/test/server";

const AUTHZ_ENDPOINT = "*/api/v1/oauth/authorizations/:id";
const CONSENT_URL = "/oauth/consent?authz=pending-1";

const authorization = {
  client_id: "https://agent.example/oauth.json",
  client_name: "Agent",
  client_uri: "https://agent.example",
  scopes_requested: ["public:read", "team:read"],
  scopes_granted: ["public:read"],
};

function mountAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe("the consent page", () => {
  const assign = vi.fn();

  beforeEach(() => {
    // jsdom cannot navigate; the page hands the browser to redirect_to.
    vi.stubGlobal("location", {
      origin: window.location.origin,
      href: window.location.href,
      assign,
    });
    server.use(
      http.get(AUTHZ_ENDPOINT, ({ params }) =>
        params.id === "pending-1"
          ? HttpResponse.json(authorization)
          : HttpResponse.json(
              { error: "authorization not found" },
              { status: 404 },
            ),
      ),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    assign.mockReset();
    sessionStorage.clear();
  });

  it("shows the client, its link, and which scopes it will get", async () => {
    mountAt(CONSENT_URL);

    const link = await screen.findByTestId("consent-client-link");
    expect(link).toHaveTextContent("Agent");
    expect(link).toHaveAttribute("href", "https://agent.example");
    expect(screen.getAllByTestId("consent-scope-granted")).toHaveLength(1);
    expect(screen.getAllByTestId("consent-scope-withheld")).toHaveLength(1);
    expect(
      screen.getByText("client: https://agent.example/oauth.json"),
    ).toBeInTheDocument();
  });

  it("allow posts the decision and sends the browser to redirect_to", async () => {
    let posted: unknown;
    server.use(
      http.post(AUTHZ_ENDPOINT, async ({ request }) => {
        posted = await request.json();
        return HttpResponse.json({
          redirect_to: "https://agent.example/cb?code=c&state=s&iss=i",
        });
      }),
    );
    mountAt(CONSENT_URL);
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Allow" }));

    await waitFor(() => {
      expect(assign).toHaveBeenCalledWith(
        "https://agent.example/cb?code=c&state=s&iss=i",
      );
    });
    expect(posted).toEqual({ decision: "allow" });
  });

  it("deny posts deny and follows the access_denied redirect", async () => {
    let posted: unknown;
    server.use(
      http.post(AUTHZ_ENDPOINT, async ({ request }) => {
        posted = await request.json();
        return HttpResponse.json({
          redirect_to: "https://agent.example/cb?error=access_denied&state=s",
        });
      }),
    );
    mountAt(CONSENT_URL);
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Deny" }));

    await waitFor(() => {
      expect(assign).toHaveBeenCalledWith(
        "https://agent.example/cb?error=access_denied&state=s",
      );
    });
    expect(posted).toEqual({ decision: "deny" });
  });

  it("a 404 is the unknown-or-expired panel, and so is a missing id", async () => {
    mountAt("/oauth/consent?authz=gone");
    expect(await screen.findByTestId("consent-missing")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Allow" }),
    ).not.toBeInTheDocument();
  });

  it("a 401 stashes the consent route and lands on /login", async () => {
    server.use(
      http.get(AUTHZ_ENDPOINT, () =>
        HttpResponse.json(
          { error: "authentication required" },
          { status: 401 },
        ),
      ),
    );
    const router = mountAt(CONSENT_URL);

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    expect(sessionStorage.getItem("docz:auth:return-to")).toBe(CONSENT_URL);
    expect(assign).not.toHaveBeenCalled();
  });

  it("a failed decision stays on the page with the error", async () => {
    server.use(
      http.post(AUTHZ_ENDPOINT, () =>
        HttpResponse.json(
          { error: "authorization not found" },
          { status: 404 },
        ),
      ),
    );
    mountAt(CONSENT_URL);
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Allow" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "authorization not found",
    );
    expect(assign).not.toHaveBeenCalled();
  });
});

describe("describeScope", () => {
  it("words a scope for a reader", () => {
    expect(describeScope("public:read")).toBe(
      "Read every repository that is public to signed-in people",
    );
    expect(describeScope("team:read")).toBe(
      'Read the repositories shared with "team"',
    );
    expect(describeScope("odd")).toBe("odd");
  });
});
