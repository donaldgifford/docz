import { SidebarMenuButton } from "@donaldgifford/design-system/components/sidebar";
import { AvatarMark } from "@donaldgifford/design-system/shell/sidebar";
import { IconLogin2 } from "@tabler/icons-react";
import { lazy, Suspense } from "react";
import { Link } from "react-router";

import { useGetSession } from "@/api/__generated__/docz-api";
import { classifySession, type SessionState } from "@/lib/session";

/*
 * The session's place at the foot of the shell's sidebar (DESIGN-0028
 * §6; classification per DESIGN-0003). classifySession drives it:
 * pending → inert placeholder, signed-out (a real 401, the ONLY route
 * to "Sign in") → the login link, anonymous (none-mode) → no footer at
 * all, unavailable (503 or any other failure) → the same inert
 * placeholder — never the login link, the state is unknown, not logged
 * out — and signed-in → the account menu (account-menu.tsx).
 *
 * AppShell reads the state through useSessionState and leaves the
 * footer out for an anonymous session, so the sidebar draws no empty
 * foot.
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

/**
 * The session as the shell shows it. Recovery from `unavailable`
 * (DESIGN-0003 OQ-2a): the shell mounts once and never unmounts, and
 * the client sets refetchOnWindowFocus: false — without a nudge, an
 * exhausted-retries failure would pin the placeholder until a full
 * reload. Re-poll gently only while the query is errored; a healthy
 * query never polls.
 */
export function useSessionState(): SessionState {
  const sessionQuery = useGetSession({
    query: {
      refetchInterval: (query) =>
        query.state.status === "error" ? SESSION_RETRY_MS : false,
    },
  });
  return classifySession(sessionQuery);
}

/** The avatar's footprint, so the footer does not shift when it lands. */
function InertPlaceholder({ testid }: { testid: string }) {
  return (
    <div aria-hidden data-testid={testid} className="flex h-12 items-center">
      <AvatarMark initials="·" className="bg-muted text-muted-foreground" />
    </div>
  );
}

export function SessionMenu({ state }: { state: SessionState }) {
  if (state.kind === "anonymous") {
    // Auth is disabled (AUTH_PROVIDERS=none) — no identity, no sign-in,
    // no chrome at all.
    return null;
  }
  if (state.kind === "pending") {
    return <InertPlaceholder testid="session-pending" />;
  }
  if (state.kind === "unavailable") {
    // Visually identical to pending (OQ-4a): the outage story belongs
    // to route surfaces; the footer's job is just to not lie.
    return <InertPlaceholder testid="session-unavailable" />;
  }
  if (state.kind === "signed-out") {
    return (
      <SidebarMenuButton
        tooltip="Sign in"
        render={<Link to="/login" data-testid="session-sign-in" />}
      >
        <IconLogin2 aria-hidden />
        <span>Sign in</span>
      </SidebarMenuButton>
    );
  }

  return (
    // Still pending as far as the footer shows: the session is known,
    // its menu is on the way.
    <Suspense fallback={<InertPlaceholder testid="session-pending" />}>
      <AccountMenu session={state.session} />
    </Suspense>
  );
}
