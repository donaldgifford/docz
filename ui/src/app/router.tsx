import { createBrowserRouter, type RouteObject } from "react-router";

import { AppShell } from "@/app/AppShell";
import { RouteErrorBoundary } from "@/app/route-error";

// Route table per DESIGN-0001 "IA and routes". Route modules load via
// route-level lazy() and export a named `Component`.
// Exported separately so tests can mount it in a memory router.
export const routes: RouteObject[] = [
  {
    path: "/",
    Component: AppShell,
    // Lazy children resolve during the router's first render; nothing
    // meaningful to paint until then.
    HydrateFallback: () => null,
    children: [
      {
        // Pathless layout route (renders a bare Outlet) whose only job
        // is to own the error boundary (DESIGN-0006 Component 9).
        //
        // It has to sit BELOW AppShell, not on the root route: a
        // boundary replaces the element of the route that OWNS it, so
        // putting it on `path: "/"` would swap out AppShell itself and
        // take the topbar with it — leaving the user stranded on the
        // panel with nothing to navigate by, which is the failure this
        // is meant to fix. Here the panel renders through AppShell's
        // Outlet and the chrome survives. A route test pins that.
        ErrorBoundary: RouteErrorBoundary,
        children: [
          { index: true, lazy: () => import("@/routes/directory") },
          { path: "login", lazy: () => import("@/routes/login") },
          { path: "repos", lazy: () => import("@/routes/repos") },
          { path: ":owner/:repo", lazy: () => import("@/routes/repo-home") },
          // Static segment — outranks `:type` in route ranking, so
          // `changelog` is a reserved word: a doc type literally named
          // "changelog" stays reachable via its id_prefix/alias URL
          // (DESIGN-0002, INV-0005 OQ-2a).
          {
            path: ":owner/:repo/changelog",
            lazy: () => import("@/routes/repo-changelog"),
          },
          // Static segment — `pages` joins `changelog` as a reserved
          // word (DESIGN-0004): the splat is a published page path (may
          // contain "/"); an empty splat redirects to the repo home,
          // because the landing page IS the repo home. A doc type
          // literally named "pages" stays reachable via its
          // id_prefix/alias URL.
          {
            path: ":owner/:repo/pages/*",
            lazy: () => import("@/routes/page"),
          },
          {
            path: ":owner/:repo/:type",
            lazy: () => import("@/routes/repo-type"),
          },
          {
            path: ":owner/:repo/:type/:docId",
            lazy: () => import("@/routes/doc"),
          },
          { path: "*", lazy: () => import("@/routes/not-found") },
        ],
      },
    ],
  },
];

export const router = createBrowserRouter(routes);
