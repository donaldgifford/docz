import { useEffect, useState, type CSSProperties } from "react";
import {
  Link,
  NavLink,
  Outlet,
  useLocation,
  useMatches,
  useNavigate,
} from "react-router";

import { useGetSession } from "@/api/__generated__/docz-api";
import { AppearanceControls } from "@/components/appearance-controls";
import { CommandPalette } from "@/components/command-palette";
import { SessionMenu } from "@/components/session-menu";
import { useAppearance } from "@/lib/appearance";
import { typeColor } from "@/lib/colors";
import { peekReturnTo, takeReturnTo } from "@/lib/authReturn";
import { refreshMermaidTheme } from "@/lib/mermaid-theme";
import { enabledNavLinks } from "@/lib/navLinks";

/**
 * The accent of a type's routes (DESIGN-0026 §8): a type page and its
 * documents take the type's colour, the one its directory filter chip
 * shows, as --type-accent, and the shell's accent follows it, so the logo,
 * the breadcrumb's leaf, the active sidebar item, and the repository mark
 * read it. Every other route keeps the shell's own accent.
 */
function typeAccentStyle(type: string | undefined): CSSProperties | undefined {
  if (type === undefined) {
    return undefined;
  }
  return {
    "--type-accent": typeColor(type),
    "--color-accent": "var(--type-accent)",
    "--color-accent-bg":
      "color-mix(in srgb, var(--type-accent) 10%, transparent)",
    "--color-accent-border":
      "color-mix(in srgb, var(--type-accent) 30%, transparent)",
  } as CSSProperties;
}

function navLinkClass({ isActive }: { isActive: boolean }): string {
  return isActive
    ? "text-fg-primary"
    : "text-fg-tertiary hover:text-fg-primary";
}

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

export function AppShell() {
  const [paletteOpen, setPaletteOpen] = useState(false);
  // Deployment-chosen pins (DESIGN-0002 Component 1) — validated by
  // navLinks.ts, so every href is a same-origin app path. Read per
  // render (cheap, ≤6 entries) so tests can vary the injected config.
  const pins = enabledNavLinks();
  // The shell's mode (DESIGN-0026 §2); a change redraws every mounted
  // diagram from the new tokens.
  const appearance = useAppearance(undefined, refreshMermaidTheme);
  const type = useMatches()
    .map((match) => match.params.type)
    .find((param) => param !== undefined);
  return (
    // display: contents, so the wrapper only carries the accent.
    <div className="contents" style={typeAccentStyle(type)}>
      <header className="sticky top-0 z-50 flex h-10 items-center gap-6 border-b border-border-default bg-bg-raised px-4">
        <Link to="/" className="flex items-center gap-[0.55rem]">
          <span
            aria-hidden
            className="grid size-[20px] place-items-center rounded-sm bg-accent font-mono text-[12px] font-bold text-(--color-on-accent)"
          >
            D
          </span>
          <span className="text-[14px] font-semibold">docz</span>
          <span className="text-[14px] text-fg-muted">· reader</span>
        </Link>

        {/* Search affordance: opens the ⌘K palette. Hidden on narrow
            viewports like the mockup. */}
        <button
          type="button"
          onClick={() => {
            setPaletteOpen(true);
          }}
          className="ml-4 hidden min-w-[260px] cursor-pointer items-center gap-2 border border-border-default px-[0.6rem] py-[0.2rem] text-[13px] text-fg-tertiary hover:border-border-strong min-[720px]:flex rounded-sm"
        >
          <svg
            aria-hidden
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <span>Search docs, rfcs, authors…</span>
          <span aria-hidden className="ml-auto flex gap-[3px]">
            <kbd className="min-w-4 border border-border-default px-1 text-center font-mono text-[12px] text-fg-tertiary rounded-sm">
              ⌘
            </kbd>
            <kbd className="min-w-4 border border-border-default px-1 text-center font-mono text-[12px] text-fg-tertiary rounded-sm">
              K
            </kbd>
          </span>
        </button>

        <nav className="ml-auto flex items-center gap-[1.4rem] text-[13px]">
          <NavLink to="/" end className={navLinkClass}>
            Directory
          </NavLink>
          <NavLink to="/repos" className={navLinkClass}>
            Repos
          </NavLink>
          {pins.map((pin) => (
            <NavLink
              key={`${pin.label}|${pin.href}`}
              to={pin.href}
              className={navLinkClass}
            >
              {pin.label}
            </NavLink>
          ))}
          <AppearanceControls
            mode={appearance.mode}
            onToggleMode={appearance.toggleMode}
          />
          <SessionMenu />
        </nav>
      </header>
      <RestoreAfterLogin />
      <Outlet />
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
    </div>
  );
}
