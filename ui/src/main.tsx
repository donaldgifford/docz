import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

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
