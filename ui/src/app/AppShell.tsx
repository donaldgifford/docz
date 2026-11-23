import { Separator } from "@donaldgifford/design-system/components/separator";
import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@donaldgifford/design-system/components/sidebar";
import { ModeToggle } from "@donaldgifford/design-system/shell/mode-toggle";
import { SearchTrigger } from "@donaldgifford/design-system/shell/search-trigger";
import {
  AppMark,
  ShellSidebar,
  ShellSidebarGroup,
} from "@donaldgifford/design-system/shell/sidebar";
import { TopBarLayout } from "@donaldgifford/design-system/shell/top-bar";
import {
  IconBook2,
  IconFolders,
  IconListSearch,
  type TablerIcon,
} from "@tabler/icons-react";
import { lazy, Suspense, useEffect, useState } from "react";
import {
  Link,
  Outlet,
  useLocation,
  useMatches,
  useNavigate,
} from "react-router";

import { useGetSession } from "@/api/__generated__/docz-api";
import {
  LazyCommandPalette,
  preloadCommandPalette,
} from "@/components/command-palette-lazy";
import { SessionMenu, useSessionState } from "@/components/session-menu";
import { TopBarSlotContext } from "@/components/top-bar-slot";
import { useAppearance } from "@/lib/appearance";
import { peekReturnTo, takeReturnTo } from "@/lib/authReturn";
import { refreshMermaidTheme } from "@/lib/diagram-theme";
import { enabledNavLinks } from "@/lib/navLinks";

/*
 * The frame every route but /login and /oauth/consent renders in
 * (DESIGN-0028 §6): the design system's sidebar, full height, and
 * beside it its top bar over the route.
 *
 *   sidebar   the docz mark; Browse (Directory, Repositories); Pinned,
 *             the deployment's nav links, absent when there are none;
 *             on a repository's routes that repository's groups
 *             (repo-sidebar.tsx); the session at its foot.
 *   top bar   the sidebar's trigger, the route's breadcrumbs (a slot a
 *             repository's pages portal into), search, and the mode
 *             toggle.
 *
 * The package keeps the sidebar's open state and a tree's open branches
 * in React state alone: no cookie and no localStorage, so a reload
 * opens the sidebar again and the server has nothing of it to forward
 * or log. Its ⌘B / Ctrl+B shortcut, the icon rail, and the sheet below
 * 768px come with SidebarProvider.
 *
 * A type's colour is not the frame's (DESIGN-0028 question 3): it is on
 * the type row's dot here, on a Dot or a Badge docz hands a tone to,
 * and inside the document. The mark, the breadcrumbs, and the active
 * row are the package's colours on every route.
 */

/**
 * The other half of SessionRequiredRedirect's stash: docz-api's OAuth
 * callback always lands on "/", so when the shell finds a stashed
 * destination there it probes getSession and, only once authenticated,
 * replaces "/" with the stash. A signed-out visit leaves the stash
 * alone (the probe 401s) — it restores on the next successful login.
 */
function RestoreAfterLogin() {
  const location = useLocation();
  const navigate = useNavigate();
  const armed = location.pathname === "/" && peekReturnTo() !== null;
  const session = useGetSession({ query: { enabled: armed } });
  const authenticated = armed && session.data?.status === 200;

  useEffect(() => {
    if (!authenticated) {
      return;
    }
    const returnTo = takeReturnTo();
    if (returnTo !== null) {
      void navigate(returnTo, { replace: true });
    }
  }, [authenticated, navigate]);

  return null;
}

/*
 * A repository's groups are a chunk of their own. They draw on that
 * repository's routes only and only from its data, so they are never
 * the first paint, and the tree under them brings Base UI's popover,
 * collapsible, and scroll area, which the rest of the frame does not
 * need (scripts/bundle-budget.ts).
 */
const RepoSidebar = lazy(async () => ({
  default: (await import("@/components/repo-sidebar")).RepoSidebar,
}));

/**
 * One of the app's own rows: a link, its icon the whole of it on the
 * icon rail with the label as the tooltip. A row with no icon (a pin)
 * shows its first letter there.
 */
function NavRow({
  label,
  to,
  icon: Icon,
  active,
}: {
  label: string;
  to: string;
  icon?: TablerIcon;
  active: boolean;
}) {
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        isActive={active}
        tooltip={label}
        aria-current={active ? "page" : undefined}
        render={<Link to={to} />}
      >
        {Icon === undefined ? (
          <span
            aria-hidden
            className="flex size-4 shrink-0 items-center justify-center rounded-sm bg-sidebar-accent text-label font-medium text-sidebar-accent-foreground uppercase"
          >
            {label.trim().charAt(0)}
          </span>
        ) : (
          <Icon aria-hidden />
        )}
        <span className="truncate">{label}</span>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/** The path a pin points at, without its query or fragment. */
function pinPath(href: string): string {
  return href.split(/[?#]/, 1)[0] ?? href;
}

export function AppShell() {
  const [paletteOpen, setPaletteOpen] = useState(false);
  // The element the route's breadcrumbs portal into; state, not a ref,
  // so the route renders them once the top bar has mounted.
  const [topBarSlot, setTopBarSlot] = useState<HTMLElement | null>(null);
  // Deployment-chosen pins (DESIGN-0002 Component 1) — validated by
  // navLinks.ts, so every href is a same-origin app path. Read per
  // render (cheap, ≤6 entries) so tests can vary the injected config.
  const pins = enabledNavLinks();
  // The shell's mode (DESIGN-0026 §2); a change redraws every mounted
  // diagram from the new tokens.
  const appearance = useAppearance(refreshMermaidTheme);
  const session = useSessionState();
  const { pathname } = useLocation();
  // The deepest match carries every param of the route in force.
  const params = useMatches().at(-1)?.params ?? {};
  const { owner, repo } = params;

  return (
    <SidebarProvider>
      <ShellSidebar
        label="docz navigation"
        header={
          <SidebarMenuButton size="lg" tooltip="docz" render={<Link to="/" />}>
            <AppMark icon={IconBook2} />
            <span className="grid flex-1 text-left leading-tight">
              <span className="truncate font-medium text-sidebar-foreground">
                docz
              </span>
              <span className="truncate text-xs text-muted-foreground">
                reader
              </span>
            </span>
          </SidebarMenuButton>
        }
        footer={
          session.kind === "anonymous" ? undefined : (
            <SessionMenu state={session} />
          )
        }
      >
        <ShellSidebarGroup label="Browse">
          <NavRow
            label="Directory"
            to="/"
            icon={IconListSearch}
            active={pathname === "/"}
          />
          <NavRow
            label="Repositories"
            to="/repos"
            icon={IconFolders}
            active={pathname === "/repos"}
          />
        </ShellSidebarGroup>
        {pins.length > 0 && (
          <ShellSidebarGroup label="Pinned">
            {pins.map((pin) => (
              <NavRow
                key={`${pin.label}|${pin.href}`}
                label={pin.label}
                to={pin.href}
                active={pathname === pinPath(pin.href)}
              />
            ))}
          </ShellSidebarGroup>
        )}
        {owner !== undefined && repo !== undefined && (
          <Suspense fallback={null}>
            <RepoSidebar
              // A fresh one per repository: its trees' open branches
              // are that repository's.
              key={`${owner}/${repo}`}
              owner={owner}
              name={repo}
              type={params.type}
              docId={params.docId}
              pagePath={params["*"]}
              pathname={pathname}
            />
          </Suspense>
        )}
      </ShellSidebar>

      {/* The package's SidebarInset is a <main>, and each route brings
          its own, so the column beside the sidebar is a plain box. */}
      <div className="relative flex min-w-0 flex-1 flex-col bg-background">
        <TopBarLayout
          start={
            <>
              <SidebarTrigger />
              <Separator
                orientation="vertical"
                className="mr-1 data-[orientation=vertical]:h-4"
              />
              <div ref={setTopBarSlot} className="min-w-0 flex-1" />
            </>
          }
          end={
            <>
              <SearchTrigger
                label="Search documents"
                shortcut="⌘K"
                aria-keyshortcuts="Meta+K Control+K"
                onClick={() => {
                  setPaletteOpen(true);
                }}
                onPointerEnter={preloadCommandPalette}
                onFocus={preloadCommandPalette}
              />
              <ModeToggle
                mode={appearance.mode}
                onToggle={appearance.toggleMode}
              />
            </>
          }
        />
        <RestoreAfterLogin />
        <TopBarSlotContext value={topBarSlot}>
          <Outlet />
        </TopBarSlotContext>
      </div>
      <LazyCommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
    </SidebarProvider>
  );
}
