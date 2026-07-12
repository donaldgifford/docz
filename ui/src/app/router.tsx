import { createBrowserRouter, type RouteObject } from "react-router";

import { AppShell } from "@/app/AppShell";

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
      { index: true, lazy: () => import("@/routes/directory") },
      { path: "login", lazy: () => import("@/routes/login") },
      { path: "repos", lazy: () => import("@/routes/repos") },
      { path: ":owner/:repo", lazy: () => import("@/routes/repo-home") },
      { path: ":owner/:repo/:type", lazy: () => import("@/routes/repo-type") },
      {
        path: ":owner/:repo/:type/:docId",
        lazy: () => import("@/routes/doc"),
      },
      { path: "*", lazy: () => import("@/routes/not-found") },
    ],
  },
];

export const router = createBrowserRouter(routes);
