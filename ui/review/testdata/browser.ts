// A stand-in for the person at the page: reads the session the way the
// page does and posts what a test tells it to, through the real server.
import type { ReviewSession } from "@/review/api";

export type Act = (
  session: ReviewSession,
  post: (path: string, body: unknown) => Promise<Response>,
) => Promise<void>;

export function fakeBrowser(act: Act): (url: string) => Promise<boolean> {
  return async (url) => {
    const u = new URL(url);
    const token = u.searchParams.get("t") ?? "";
    const headers = { Authorization: `Bearer ${token}` };
    const session = (await (
      await fetch(`${u.origin}/api/review`, { headers })
    ).json()) as ReviewSession;
    const post = (path: string, body: unknown) =>
      fetch(`${u.origin}${path}`, {
        method: "POST",
        headers: {
          ...headers,
          Origin: u.origin,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      });
    // The page acts after the opener returns, as a real browser would.
    setTimeout(() => void act(session, post), 0);
    return true;
  };
}

/** The key fields of a mutation against document i of the session. */
export function keyOf(
  session: ReviewSession,
  i: number,
  submission_id: string,
) {
  const d = session.documents[i];
  return {
    round: d?.round,
    content_sha256: d?.inspect.content_sha256,
    submission_id,
  };
}
