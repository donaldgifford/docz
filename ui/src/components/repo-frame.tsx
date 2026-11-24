import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@donaldgifford/design-system/components/breadcrumb";
import { Fragment, use, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router";

import { TopBarSlotContext } from "@/components/top-bar-slot";

/*
 * The frame of a repository's pages inside the shell (DESIGN-0028 §6):
 * the content, and a 190px rail to its right from 1181px. The
 * repository's navigation is the shell's sidebar (repo-sidebar.tsx) and
 * the breadcrumbs sit in the shell's top bar, so every repo-scoped page
 * — home, changelog, pages, type, reader — mounts inside this and hands
 * it its crumbs and its rail.
 */

export interface Crumb {
  label: string;
  /** Link target; the last crumb, the current page, needs none. */
  to?: string;
}

/**
 * The route's breadcrumbs, drawn in the shell's top bar: the design
 * system's Breadcrumb, portalled into the slot AppShell provides. It
 * draws nothing where there is no top bar.
 */
export function RepoBreadcrumbs({ crumbs }: { crumbs: Crumb[] }) {
  const slot = use(TopBarSlotContext);
  if (slot === null) {
    return null;
  }
  return createPortal(
    // The ancestors keep their width and the leaf gives way; below md
    // the leaf is all there is room for.
    <Breadcrumb aria-label="Breadcrumb" className="min-w-0">
      <BreadcrumbList className="flex-nowrap font-mono">
        <BreadcrumbItem className="shrink-0 max-md:hidden">
          <BreadcrumbLink render={<Link to="/repos" />}>repos</BreadcrumbLink>
        </BreadcrumbItem>
        {crumbs.map((crumb, index) => (
          <Fragment key={`${crumb.label}:${crumb.to ?? ""}`}>
            <BreadcrumbSeparator className="max-md:hidden" />
            {index === crumbs.length - 1 ? (
              <BreadcrumbItem className="min-w-0">
                <BreadcrumbPage className="truncate">
                  {crumb.label}
                </BreadcrumbPage>
              </BreadcrumbItem>
            ) : (
              <BreadcrumbItem className="shrink-0 max-md:hidden">
                {crumb.to === undefined ? (
                  // A step with nothing of its own to open: `pages`,
                  // a directory no page publishes at.
                  crumb.label
                ) : (
                  <BreadcrumbLink render={<Link to={crumb.to} />}>
                    {crumb.label}
                  </BreadcrumbLink>
                )}
              </BreadcrumbItem>
            )}
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>,
    slot,
  );
}

export function RepoFrame({
  crumbs,
  rail,
  children,
}: {
  crumbs: Crumb[];
  /** Right-rail content; omit to drop the second column's content. */
  rail?: ReactNode;
  children: ReactNode;
}) {
  return (
    // 1062px is the content's 784px measure, the rail, and the gap.
    <div className="mx-auto grid w-full max-w-[1062px] grid-cols-1 px-5 py-7 min-[1181px]:grid-cols-[minmax(0,1fr)_190px] min-[1181px]:gap-x-12">
      <RepoBreadcrumbs crumbs={crumbs} />
      <main className="min-w-0">{children}</main>

      <aside className="hidden min-[1181px]:block">
        <div className="sticky top-[64px]">{rail}</div>
      </aside>
    </div>
  );
}
