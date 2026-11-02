import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router";

// The site's self-hosted fonts and its one stylesheet (src/main.tsx).
import "@fontsource-variable/ibm-plex-sans/wght.css";
import "@fontsource-variable/ibm-plex-sans/wght-italic.css";
import "@fontsource-variable/jetbrains-mono/wght.css";
import "@fontsource-variable/jetbrains-mono/wght-italic.css";
import "@fontsource-variable/source-serif-4/wght.css";
import "@fontsource-variable/source-serif-4/wght-italic.css";

import "@/theme/tokens.css";

import {
  createReviewClient,
  type ReviewClient,
  type ReviewSession,
  takeToken,
} from "@/review/api";
import { applyStoredMode } from "@/lib/appearance";
import { ReviewApp, SESSION_KEY } from "@/review/app";

/*
 * docz-review's page entry (review.html). The token is taken from the
 * URL before anything renders and lives only in the client's closure.
 * The session is fetched before the first render too, so the document
 * column paints in the repository's theme (DESIGN-0025 §3) rather than
 * flashing paper first; it seeds the query cache, so the app does not
 * ask again. A failed fetch renders anyway and the app reports it.
 * The rendered markdown's links expect a router, so a MemoryRouter
 * stands in: the page has no routes of its own.
 */

applyStoredMode();
const token = takeToken();
const root = document.getElementById("root");
if (root === null) {
  throw new Error("review.html is missing the #root mount point");
}

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});

async function firstSession(
  client: ReviewClient,
): Promise<ReviewSession | null> {
  try {
    return await client.session();
  } catch {
    return null;
  }
}

async function start(mount: HTMLElement): Promise<void> {
  const client = token === null ? null : createReviewClient(token);
  if (client !== null) {
    const session = await firstSession(client);
    if (session !== null) {
      queryClient.setQueryData(SESSION_KEY, session);
    }
  }

  createRoot(mount).render(
    <StrictMode>
      {client === null ? (
        <p role="alert" className="p-8 font-mono text-st-rejected">
          This page needs the link docz-review printed; open that link instead.
        </p>
      ) : (
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <ReviewApp client={client} />
          </MemoryRouter>
        </QueryClientProvider>
      )}
    </StrictMode>,
  );
}

void start(root);
