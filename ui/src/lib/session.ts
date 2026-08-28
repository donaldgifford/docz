import type { getSessionResponse } from "@/api/__generated__/docz-api";
import type { Session } from "@/api/__generated__/docz-api.schemas";
import { SessionRequiredError } from "@/api/fetcher";

/*
 * Session classification (DESIGN-0003): the one place the session
 * query's raw states become auth-chrome meaning. Both consumers
 * (SessionMenu, /login) render from this, so the invariants live here:
 * "signed-out" only ever comes from a real 401, and every other
 * failure — the session gate's 503 included — is "unavailable"
 * (unknown, NOT signed out; nothing may treat it as a logout).
 */

export type SessionState =
  | { kind: "pending" }
  | { kind: "signed-in"; session: Session }
  /** `provider === "none"`: auth is disabled (AUTH_PROVIDERS=none). */
  | { kind: "anonymous" }
  /** The API answered 401 — there really is no session. */
  | { kind: "signed-out" }
  /** 503 or any other failure — state unknown, never a logout. */
  | { kind: "unavailable" };

/** The slice of a `useGetSession()` result the classifier reads. */
interface SessionQueryLike {
  isPending: boolean;
  error: unknown;
  data: getSessionResponse | undefined;
}

export function classifySession(query: SessionQueryLike): SessionState {
  if (query.isPending) {
    return { kind: "pending" };
  }
  if (query.error !== null && query.error !== undefined) {
    return query.error instanceof SessionRequiredError
      ? { kind: "signed-out" }
      : { kind: "unavailable" };
  }
  if (query.data?.status === 200) {
    return query.data.data.provider === "none"
      ? { kind: "anonymous" }
      : { kind: "signed-in", session: query.data.data };
  }
  // Settled, no error, yet no 200 envelope — nothing the contract
  // produces, so treat it as unknown rather than signed out.
  return { kind: "unavailable" };
}
