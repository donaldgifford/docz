import { createBrowserRouter } from "react-router";

import { AppShell } from "@/app/AppShell";

// Route table per DESIGN-0001 "IA and routes". Route modules load via
// route-level lazy() and export a named `Component`; /login is post-MVP.
export const router = createBrowserRouter([
  {
    path: "/",
    Component: AppShell,
    children: [
      { index: true, lazy: () => import("@/routes/directory") },
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
]);
