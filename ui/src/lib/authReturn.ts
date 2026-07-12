/*
 * 401 → login round-trip stash (DESIGN-0001 auth flow, IMPL-0001
 * Phase 5). docz-api's OAuth callback always lands the browser back on
 * "/", so the intended destination survives the round trip in
 * sessionStorage. Only a same-origin app path ever goes in — never a
 * token; the docz_session cookie is httpOnly and invisible to JS.
 */

const RETURN_KEY = "docz:auth:return-to";

/*
 * The restore navigates blind, so both ends validate: app paths only
 * (rejects absolute and protocol-relative "//host" URLs — open
 * redirect), and never "/" or "/login" (would loop the round trip).
 */
function isStashable(path: string): boolean {
  return (
    path.startsWith("/") &&
    !path.startsWith("//") &&
    path !== "/" &&
    path !== "/login" &&
    !path.startsWith("/login?")
  );
}

export function stashReturnTo(path: string): void {
  if (!isStashable(path)) {
    return;
  }
  try {
    sessionStorage.setItem(RETURN_KEY, path);
  } catch {
    // Storage disabled — lose the destination, keep the redirect.
  }
}

/** Non-destructive read; gates the restore's session probe. */
export function peekReturnTo(): string | null {
  try {
    const path = sessionStorage.getItem(RETURN_KEY);
    return path !== null && isStashable(path) ? path : null;
  } catch {
    return null;
  }
}

/** Read-and-clear, so a restore fires at most once per stash. */
export function takeReturnTo(): string | null {
  const path = peekReturnTo();
  try {
    sessionStorage.removeItem(RETURN_KEY);
  } catch {
    // Storage disabled — peek already returned null.
  }
  return path;
}
