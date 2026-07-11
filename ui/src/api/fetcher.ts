/*
 * Custom fetch mutator for the orval-generated client, and the typed
 * errors the rest of the app matches on. Same-origin by design: docz-api
 * serves no CORS headers, and auth is its httpOnly docz_session cookie —
 * the site never sees or stores a token.
 */

/** docz-api error envelope: `{ "error": "human-readable message" }`. */
interface ErrorEnvelope {
  error: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly url: string;

  constructor(message: string, status: number, url: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.url = url;
  }
}

/** 401 — no valid docz_session cookie; UI shows the session panel. */
export class SessionRequiredError extends ApiError {
  constructor(message: string, url: string) {
    super(message, 401, url);
    this.name = "SessionRequiredError";
  }
}

/** 404 — also how docz-api hides repos the session can't access. */
export class NotFoundError extends ApiError {
  constructor(message: string, url: string) {
    super(message, 404, url);
    this.name = "NotFoundError";
  }
}

function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
  return (
    typeof value === "object" &&
    value !== null &&
    "error" in value &&
    typeof (value as { error: unknown }).error === "string"
  );
}

async function toApiError(response: Response, url: string): Promise<ApiError> {
  let message = `${String(response.status)} ${response.statusText}`;
  try {
    const body: unknown = await response.json();
    if (isErrorEnvelope(body)) {
      message = body.error;
    }
  } catch {
    // Non-JSON error body — keep the status-line message.
  }

  switch (response.status) {
    case 401:
      return new SessionRequiredError(message, url);
    case 404:
      return new NotFoundError(message, url);
    default:
      return new ApiError(message, response.status, url);
  }
}

export async function fetcher<T>(url: string, options?: RequestInit): Promise<T> {
  // The generated client passes same-origin paths ("/api/v1/…"). In the
  // browser that's already absolute enough; under jsdom (tests) Node's
  // fetch rejects relative URLs, so resolve against the window origin.
  const resolvedUrl =
    url.startsWith("/") && typeof window !== "undefined"
      ? new URL(url, window.location.origin).toString()
      : url;

  const response = await fetch(resolvedUrl, {
    credentials: "same-origin",
    ...options,
    headers: {
      Accept: "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    throw await toApiError(response, url);
  }

  // Orval's fetch client types responses as an envelope of the parsed
  // body plus status/headers — the mutator is responsible for building
  // it. Errors never reach this shape; they throw above.
  const text = response.status === 204 ? "" : await response.text();
  const data: unknown = text === "" ? undefined : JSON.parse(text);
  return {
    data,
    status: response.status,
    headers: response.headers,
  } as T;
}

export default fetcher;
