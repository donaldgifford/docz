import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";

import { useGetSession, useLogout } from "@/api/__generated__/docz-api";
import { classifySession } from "@/lib/session";

/*
 * Session-aware topbar chrome (DESIGN-0001 auth, IMPL-0001 Phase 5;
 * classification per DESIGN-0003). classifySession drives the render:
 * pending → inert placeholder, signed-out (a real 401, the ONLY route
 * to "Sign in") → the login link, anonymous (none-mode) → nothing,
 * unavailable (503 or any other failure) → the same inert placeholder
 * — never the login link, the state is unknown, not logged out —
 * and signed-in → avatar disclosure with the identity (GitHub
 * `login`, OIDC `email`) and sign-out. Deliberately a disclosure, not
 * role="menu" — one action doesn't warrant roving focus, and axe
 * flags half-implemented menu semantics.
 */

/** How often an errored session query re-probes (outage-only). */
const SESSION_RETRY_MS = 30_000;

/** Same footprint as the avatar so the topbar doesn't shift. */
function InertPlaceholder({ testid }: { testid: string }) {
  return (
    <span
      aria-hidden
      data-testid={testid}
      className="grid size-[26px] place-items-center border border-border-default bg-bg-elevated text-[11px] text-fg-secondary"
    >
      ·
    </span>
  );
}

export function SessionMenu() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();

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
  const logout = useLogout({
    mutation: {
      // Settled, not success: even a failed POST (say the cookie
      // already expired) must not strand stale per-session data — the
      // design's invariant is "nothing fetched under the old session
      // outlives it". Navigate BEFORE clearing so the page being left
      // doesn't refetch everything into the fresh cache.
      onSettled: async () => {
        await navigate("/login");
        queryClient.clear();
      },
    },
  });

  useEffect(() => {
    if (!open) {
      return;
    }
    function closeOnOutsidePress(event: PointerEvent) {
      if (
        event.target instanceof Node &&
        rootRef.current?.contains(event.target) === true
      ) {
        return;
      }
      setOpen(false);
    }
    document.addEventListener("pointerdown", closeOnOutsidePress);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePress);
    };
  }, [open]);

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
        className="text-fg-tertiary hover:text-fg-primary"
        data-testid="topbar-sign-in"
      >
        Sign in
      </Link>
    );
  }

  const { session } = state;
  const identity = session.login ?? session.email ?? session.subject;
  const initial = identity.charAt(0).toUpperCase() || "?";

  return (
    // Keyboard path: Escape dismisses the open panel and hands focus
    // back to the trigger (pointer users get pointerdown-outside above).
    // The div only catches Escape bubbling from the interactive children
    // — the rule's documented event-delegation exception.
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions
    <div
      ref={rootRef}
      className="relative"
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) {
          event.stopPropagation();
          setOpen(false);
          triggerRef.current?.focus();
        }
      }}
    >
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="true"
        aria-expanded={open}
        aria-label={`Account: ${identity}`}
        onClick={() => {
          setOpen((wasOpen) => !wasOpen);
        }}
        className="grid size-[26px] cursor-pointer place-items-center border border-border-default bg-bg-elevated text-[11px] text-fg-secondary hover:border-border-strong hover:text-fg-primary"
      >
        {initial}
      </button>
      {open && (
        <div className="absolute right-0 top-[calc(100%+8px)] z-50 min-w-[200px] border border-border-default bg-bg-raised py-2">
          <div className="px-4 pb-2">
            <div className="truncate text-[13px] text-fg-primary">
              {identity}
            </div>
            <div className="font-mono text-[11px] text-fg-muted">
              via {session.provider}
            </div>
          </div>
          <button
            type="button"
            disabled={logout.isPending}
            onClick={() => {
              logout.mutate();
            }}
            className="w-full cursor-pointer border-t border-border-default px-4 pt-2 text-left font-mono text-[12.5px] text-fg-secondary hover:text-fg-primary disabled:cursor-default disabled:text-fg-muted"
          >
            {logout.isPending ? "Signing out…" : "Sign out"}
          </button>
        </div>
      )}
    </div>
  );
}
