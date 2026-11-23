import { lazy, Suspense } from "react";
import { Link, useLocation } from "react-router";

import { useGetSession } from "@/api/__generated__/docz-api";
import { classifySession } from "@/lib/session";

/*
 * Session-aware topbar chrome (DESIGN-0001 auth, IMPL-0001 Phase 5;
 * classification per DESIGN-0003). classifySession drives the render:
 * pending → inert placeholder, signed-out (a real 401, the ONLY route
 * to "Sign in") → the login link, anonymous (none-mode) → nothing,
 * unavailable (503 or any other failure) → the same inert placeholder
 * — never the login link, the state is unknown, not logged out —
 * and signed-in → the account menu (account-menu.tsx).
 *
 * The account menu is the design system's DropdownMenu, and with it
 * comes Base UI's menu and positioning, about 30 KB gzipped. Only a
 * signed-in session draws it, and only once the session probe has
 * answered, so it is a chunk of its own and the placeholder holds its
 * place while it loads (scripts/bundle-budget.ts).
 */

const AccountMenu = lazy(async () => ({
  default: (await import("@/components/account-menu")).AccountMenu,
}));

/** How often an errored session query re-probes (outage-only). */
const SESSION_RETRY_MS = 30_000;

/** Same footprint as the avatar so the topbar doesn't shift. */
function InertPlaceholder({ testid }: { testid: string }) {
  return (
    <span
      aria-hidden
      data-testid={testid}
      className="grid size-6 place-items-center rounded-lg bg-muted text-xs text-muted-foreground"
    >
      ·
    </span>
  );
}

export function SessionMenu() {
  const location = useLocation();

  // Recovery from `unavailable` (DESIGN-0003 OQ-2a): SessionMenu
  // mounts once in AppShell and never unmounts, and the client sets
  // refetchOnWindowFocus: false — without a nudge, an exhausted-
  // retries failure would pin the placeholder until a full reload.
  // Re-poll gently only while the query is errored; a healthy query
  // never polls.
  const sessionQuery = useGetSession({
    query: {
      refetchInterval: (query) =>
        query.state.status === "error" ? SESSION_RETRY_MS : false,
    },
  });

  const state = classifySession(sessionQuery);

  if (state.kind === "pending") {
    return <InertPlaceholder testid="session-pending" />;
  }
  if (state.kind === "anonymous") {
    // Auth is disabled (AUTH_PROVIDERS=none) — no identity, no sign-in,
    // no chrome at all.
    return null;
  }
  if (state.kind === "unavailable") {
    // Visually identical to pending (OQ-4a): the outage story belongs
    // to route surfaces; the topbar's job is just to not lie.
    return <InertPlaceholder testid="session-unavailable" />;
  }
  if (state.kind === "signed-out") {
    // On /login the page itself is the sign-in affordance — a topbar
    // link pointing at the page you're already on is noise.
    if (location.pathname === "/login") {
      return null;
    }
    return (
      <Link
        to="/login"
        className="text-muted-foreground hover:text-foreground"
        data-testid="topbar-sign-in"
      >
        Sign in
      </Link>
    );
  }

  return (
    // Still pending as far as the topbar shows: the session is known,
    // its menu is on the way.
    <Suspense fallback={<InertPlaceholder testid="session-pending" />}>
      <AccountMenu session={state.session} />
    </Suspense>
  );
}
