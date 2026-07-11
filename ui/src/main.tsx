import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

// Self-hosted fonts (@fontsource) — the site must make no third-party
// font requests. Weights track actual usage in the mockup/reader.
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/ibm-plex-sans/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/600.css";
import "@fontsource/ibm-plex-mono/700.css";
import "@fontsource/source-serif-4/400.css";
import "@fontsource/source-serif-4/400-italic.css";
import "@fontsource/source-serif-4/600.css";
import "@fontsource/source-serif-4/700.css";

import "@/theme/tokens.css";

const rootElement = document.getElementById("root");
if (rootElement === null) {
  throw new Error("index.html is missing the #root mount point");
}

createRoot(rootElement).render(
  <StrictMode>
    <p className="p-4 font-mono text-fg-tertiary">
      docz-site scaffold — app shell lands in a later Phase 0 task.
    </p>
  </StrictMode>,
);
