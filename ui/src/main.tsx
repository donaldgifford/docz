import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router";

// Self-hosted fonts (@fontsource) — the site must make no third-party
// font requests. DESIGN-0005 stack as amended: Source Serif 4
// (variable) carries reader prose, its headings, and pull quotes;
// Mona Sans (variable) carries UI chrome and page titles outside the
// article; Monaspace Neon carries every mono surface, with its true
// italic so Shiki's italic scopes render as designed. Both variable
// families ship a weight axis plus a matching italic file. Keep this
// set in sync with mockup.html's <link> list.
import "@fontsource-variable/mona-sans/wght.css";
import "@fontsource-variable/mona-sans/wght-italic.css";
import "@fontsource-variable/source-serif-4/wght.css";
import "@fontsource-variable/source-serif-4/wght-italic.css";
import "@fontsource/monaspace-neon/400.css";
import "@fontsource/monaspace-neon/400-italic.css";
import "@fontsource/monaspace-neon/600.css";
import "@fontsource/monaspace-neon/700.css";

import "@/theme/tokens.css";

import { createQueryClient } from "@/app/query-client";
import { router } from "@/app/router";

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
