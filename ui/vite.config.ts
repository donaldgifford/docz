import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// docz-api serves no CORS headers — prod is same-origin (DESIGN-0001
// Decision 9). Dev mirrors that by proxying every path the SPA touches
// to a locally running docz-api (HTTP_ADDR defaults to :8080).
const doczApiUrl = process.env.DOCZ_API_URL ?? "http://localhost:8080";

/*
 * mermaid 12 ships ES2024 and states a Safari 17.4 floor (IMPL-0006
 * OQ-4). That floor exists whether or not it is written down — left to
 * Vite's default target the bundler would keep claiming support it
 * cannot deliver, and the truth would arrive as a bug report from an old
 * iPad. Stated here in browser versions rather than as `es2024` so
 * `cssTarget` inherits something meaningful, and repeated in prose in
 * README.md. These are the first versions with full ES2024 support;
 * Safari is the binding constraint.
 */
const BROWSER_FLOOR = ["chrome119", "edge119", "firefox122", "safari17.4"];

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { target: BROWSER_FLOOR },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": doczApiUrl,
      "/auth": doczApiUrl,
      "/openapi.yaml": doczApiUrl,
    },
  },
});
