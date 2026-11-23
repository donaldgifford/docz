import { createContext } from "react";

/*
 * Where a route's breadcrumbs go (DESIGN-0028 §6). The top bar is the
 * shell's, above the Outlet, and the breadcrumbs are the route's, whose
 * module is a lazy chunk the shell never imports. AppShell hands the
 * element at the top bar's start down through this context, and
 * RepoFrame portals its breadcrumbs into it.
 *
 * null before the top bar has mounted, and outside the frame (`/login`,
 * `/oauth/consent`, a component test with no shell): there is no bar to
 * draw in, and the breadcrumbs draw nothing.
 */
export const TopBarSlotContext = createContext<HTMLElement | null>(null);
