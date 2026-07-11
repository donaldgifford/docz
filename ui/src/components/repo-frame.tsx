import { Fragment, type ReactNode } from "react";
import { Link } from "react-router";

import { RepoNav } from "@/components/repo-nav";

/*
 * The mockup's three-column repo view: sticky RepoNav left, content
 * center, a 190px rail right (hidden below 1181px; the nav collapses
 * to a stacked bordered header below 861px). Every repo-scoped page —
 * home, type, reader — mounts inside this frame so breadcrumbs and
 * collapse behavior stay consistent.
 */

export interface Crumb {
  label: string;
  /** Link target; the current (last) crumb omits it and renders accent. */
  to?: string;
}

export function RepoBreadcrumbs({ crumbs }: { crumbs: Crumb[] }) {
  return (
    <nav
      aria-label="Breadcrumb"
      className="mb-6 flex flex-wrap gap-2 font-mono text-[12px] text-fg-muted"
    >
      <Link to="/repos" className="text-fg-tertiary hover:text-fg-primary">
        repos
      </Link>
      {crumbs.map((crumb) => (
        <Fragment key={`${crumb.label}:${crumb.to ?? ""}`}>
          <span>/</span>
          {crumb.to === undefined ? (
            <span className="text-accent [overflow-wrap:anywhere]">
              {crumb.label}
            </span>
          ) : (
            <Link
              to={crumb.to}
              className="text-fg-tertiary hover:text-fg-primary"
            >
              {crumb.label}
            </Link>
          )}
        </Fragment>
      ))}
    </nav>
  );
}

export function RepoFrame({
  owner,
  name,
  crumbs,
  rail,
  children,
}: {
  owner: string;
  name: string;
  crumbs: Crumb[];
  /** Right-rail content; omit to drop the third column's content. */
  rail?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="mx-auto grid max-w-[1360px] grid-cols-1 gap-y-8 px-5 py-7 min-[861px]:grid-cols-[250px_minmax(0,1fr)] min-[861px]:gap-x-10 min-[1181px]:grid-cols-[250px_minmax(0,1fr)_190px] min-[1181px]:gap-x-12">
      <div className="border-b border-border-hairline pb-5 min-[861px]:sticky min-[861px]:top-[76px] min-[861px]:max-h-[calc(100vh-6rem)] min-[861px]:self-start min-[861px]:overflow-y-auto min-[861px]:border-b-0 min-[861px]:pb-0">
        <RepoNav owner={owner} name={name} />
      </div>

      <main className="min-w-0">
        <RepoBreadcrumbs crumbs={crumbs} />
        {children}
      </main>

      <aside className="hidden min-[1181px]:block">
        <div className="sticky top-[76px]">{rail}</div>
      </aside>
    </div>
  );
}
