import { Link, NavLink, Outlet } from "react-router";

function navLinkClass({ isActive }: { isActive: boolean }): string {
  return isActive
    ? "text-fg-primary"
    : "text-fg-tertiary hover:text-fg-primary";
}

export function AppShell() {
  return (
    <>
      <header className="sticky top-0 z-50 flex h-[52px] items-center gap-6 border-b border-border-default bg-[rgba(12,16,23,0.88)] px-5 backdrop-blur-[10px]">
        <Link to="/" className="flex items-center gap-[0.55rem]">
          <span
            aria-hidden
            className="grid size-[22px] place-items-center bg-accent font-mono text-[13px] font-bold text-bg-base"
          >
            D
          </span>
          <span className="font-mono text-[14px] font-semibold tracking-[0.01em]">
            docz
          </span>
          <span className="font-mono text-[13px] text-fg-muted">· reader</span>
        </Link>

        {/* Search affordance — Phase 2 swaps this for the ⌘K palette
            trigger; until then it routes to the directory (the search
            view). Hidden on narrow viewports like the mockup. */}
        <Link
          to="/"
          className="ml-4 hidden min-w-[260px] items-center gap-2 border border-border-default px-[0.7rem] py-[0.3rem] text-[12.5px] text-fg-tertiary min-[720px]:flex"
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
            <kbd className="min-w-4 border border-border-default px-1 text-center font-mono text-[11px] text-fg-tertiary">
              ⌘
            </kbd>
            <kbd className="min-w-4 border border-border-default px-1 text-center font-mono text-[11px] text-fg-tertiary">
              K
            </kbd>
          </span>
        </Link>

        <nav className="ml-auto flex items-center gap-[1.4rem] font-mono text-[13px]">
          <NavLink to="/" end className={navLinkClass}>
            Directory
          </NavLink>
          <NavLink to="/repos" className={navLinkClass}>
            Repos
          </NavLink>
          {/* Avatar placeholder — session/avatar wiring is Phase 5. */}
          <span
            aria-hidden
            className="grid size-[26px] place-items-center border border-border-default bg-bg-elevated text-[11px] text-fg-secondary"
          >
            ·
          </span>
        </nav>
      </header>
      <Outlet />
    </>
  );
}
