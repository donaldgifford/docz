import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router";

// Self-hosted fonts (@fontsource) — the site must make no third-party
// font requests. The shell is repo-guardian's (DESIGN-0026 §2): IBM
// Plex Sans for chrome and JetBrains Mono for every mono surface, its
// true italic so Shiki's italic scopes render as designed. Source Serif
// 4 still sets markdown the reader renders itself (a document whose
// fragment failed, the repo home, the changelog, and published pages).
// Each variable family ships a weight axis and a matching italic file.
import "@fontsource-variable/ibm-plex-sans/wght.css";
import "@fontsource-variable/ibm-plex-sans/wght-italic.css";
import "@fontsource-variable/jetbrains-mono/wght.css";
import "@fontsource-variable/jetbrains-mono/wght-italic.css";
import "@fontsource-variable/source-serif-4/wght.css";
import "@fontsource-variable/source-serif-4/wght-italic.css";

import "@/theme/tokens.css";

import { createQueryClient } from "@/app/query-client";
import { router } from "@/app/router";
import { applyStoredMode } from "@/lib/appearance";

// The mode goes on <html> before anything renders, so the first paint is
// the reader's mode (DESIGN-0026 §2); with none stored, shell.css follows
// the system.
applyStoredMode();

async function bootstrap(): Promise<void> {
  if (import.meta.env.VITE_API_MODE === "msw") {
    const { worker } = await import("@/mocks/browser");
    await worker.start({ onUnhandledRequest: "bypass" });
  }

  const queryClient = createQueryClient();

  const rootElement = document.getElementById("root");
  if (rootElement === null) {
    throw new Error("index.html is missing the #root mount point");
  }

  createRoot(rootElement).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </StrictMode>,
  );
}

void bootstrap();
