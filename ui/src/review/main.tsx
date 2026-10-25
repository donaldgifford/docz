import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router";

// The site's self-hosted fonts and its one stylesheet (src/main.tsx).
import "@fontsource-variable/mona-sans/wght.css";
import "@fontsource-variable/mona-sans/wght-italic.css";
import "@fontsource-variable/source-serif-4/wght.css";
import "@fontsource-variable/source-serif-4/wght-italic.css";
import "@fontsource/monaspace-neon/400.css";
import "@fontsource/monaspace-neon/400-italic.css";
import "@fontsource/monaspace-neon/600.css";
import "@fontsource/monaspace-neon/700.css";

import "@/theme/tokens.css";

import { createReviewClient, takeToken } from "@/review/api";
import { ReviewApp } from "@/review/app";

/*
 * docz-review's page entry (review.html). The token is taken from the
 * URL before anything renders and lives only in the client's closure.
 * The rendered markdown's links expect a router, so a MemoryRouter
 * stands in: the page has no routes of its own.
 */

const token = takeToken();
const root = document.getElementById("root");
if (root === null) {
  throw new Error("review.html is missing the #root mount point");
}

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});

createRoot(root).render(
  <StrictMode>
    {token === null ? (
      <p role="alert" className="p-8 font-mono text-st-rejected">
        This page needs the link docz-review printed; open that link instead.
      </p>
    ) : (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ReviewApp client={createReviewClient(token)} />
        </MemoryRouter>
      </QueryClientProvider>
    )}
  </StrictMode>,
);
