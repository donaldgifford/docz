import { HttpResponse, http } from "msw";
import { describe, expect, it } from "vitest";

import {
  ApiError,
  NotFoundError,
  SessionRequiredError,
  SessionUnavailableError,
  fetcher,
} from "@/api/fetcher";
import { server } from "@/test/server";

const URL_PATH = "/api/v1/auth/session";

function respondWith(status: number, body: string, json = true) {
  server.use(
    http.get(`*${URL_PATH}`, () =>
      json
        ? HttpResponse.text(body, {
            status,
            headers: { "Content-Type": "application/json" },
          })
        : HttpResponse.text(body, { status }),
    ),
  );
}

async function fetchError(): Promise<ApiError> {
  try {
    await fetcher(URL_PATH);
  } catch (error) {
    if (error instanceof ApiError) return error;
    throw error;
  }
  throw new Error("fetcher resolved; expected an ApiError");
}

describe("toApiError mapping", () => {
  it("maps 503 with the docz-api envelope to SessionUnavailableError", async () => {
    respondWith(503, '{"error":"session unavailable"}');
    const error = await fetchError();
    expect(error).toBeInstanceOf(SessionUnavailableError);
    expect(error.status).toBe(503);
    expect(error.message).toBe("session unavailable");
    expect(error.name).toBe("SessionUnavailableError");
  });

  it("maps a non-JSON 503 body to the same class with the status-line message", async () => {
    respondWith(503, "upstream connect error", false);
    const error = await fetchError();
    expect(error).toBeInstanceOf(SessionUnavailableError);
    expect(error.status).toBe(503);
    expect(error.message).toBe("503 Service Unavailable");
  });

  it("a SessionUnavailableError is never a SessionRequiredError", async () => {
    // The 503-never-logout invariant at the type level: nothing matching
    // on SessionRequiredError may catch a backend outage.
    respondWith(503, '{"error":"session unavailable"}');
    const error = await fetchError();
    expect(error).not.toBeInstanceOf(SessionRequiredError);
  });

  it("keeps the 401 → SessionRequiredError mapping", async () => {
    respondWith(401, '{"error":"session required"}');
    const error = await fetchError();
    expect(error).toBeInstanceOf(SessionRequiredError);
    expect(error.status).toBe(401);
    expect(error.message).toBe("session required");
  });

  it("keeps the 404 → NotFoundError mapping", async () => {
    respondWith(404, '{"error":"not found"}');
    const error = await fetchError();
    expect(error).toBeInstanceOf(NotFoundError);
    expect(error.status).toBe(404);
    expect(error.message).toBe("not found");
  });

  it("keeps other statuses as plain ApiError", async () => {
    respondWith(500, '{"error":"boom"}');
    const error = await fetchError();
    expect(error).toBeInstanceOf(ApiError);
    expect(error).not.toBeInstanceOf(SessionRequiredError);
    expect(error).not.toBeInstanceOf(NotFoundError);
    expect(error).not.toBeInstanceOf(SessionUnavailableError);
    expect(error.status).toBe(500);
    expect(error.message).toBe("boom");
  });
});
