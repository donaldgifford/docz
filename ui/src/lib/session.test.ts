import { describe, expect, it } from "vitest";

import type { getSessionResponse } from "@/api/__generated__/docz-api";
import type { Session } from "@/api/__generated__/docz-api.schemas";
import { SessionRequiredError, SessionUnavailableError } from "@/api/fetcher";
import { classifySession } from "@/lib/session";

const URL = "/api/v1/auth/session";

function ok(data: getSessionResponse["data"]): getSessionResponse {
  return { data, status: 200, headers: new Headers() } as getSessionResponse;
}

describe("classifySession", () => {
  it("pending query → pending", () => {
    expect(
      classifySession({ isPending: true, error: null, data: undefined }),
    ).toEqual({ kind: "pending" });
  });

  it("200 with a github identity → signed-in carrying the session", () => {
    const session: Session = {
      provider: "github",
      subject: "12345",
      via: "cookie",
      scopes: ["public:read"],
      login: "donaldgifford",
    };
    expect(
      classifySession({ isPending: false, error: null, data: ok(session) }),
    ).toEqual({ kind: "signed-in", session });
  });

  it('200 with provider "none" → anonymous (auth disabled)', () => {
    expect(
      classifySession({
        isPending: false,
        error: null,
        data: ok({
          provider: "none",
          subject: "anonymous",
          via: "anonymous",
          scopes: [],
          login: "anonymous",
        }),
      }),
    ).toEqual({ kind: "anonymous" });
  });

  it("401 (SessionRequiredError) → signed-out", () => {
    expect(
      classifySession({
        isPending: false,
        error: new SessionRequiredError("session required", URL),
        data: undefined,
      }),
    ).toEqual({ kind: "signed-out" });
  });

  it("503 (SessionUnavailableError) → unavailable, never signed-out", () => {
    expect(
      classifySession({
        isPending: false,
        error: new SessionUnavailableError("session unavailable", URL),
        data: undefined,
      }),
    ).toEqual({ kind: "unavailable" });
  });

  it("generic network failure → unavailable", () => {
    expect(
      classifySession({
        isPending: false,
        error: new TypeError("Failed to fetch"),
        data: undefined,
      }),
    ).toEqual({ kind: "unavailable" });
  });
});
