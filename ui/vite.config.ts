import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, searchForWorkspaceRoot } from "vite";

import { reviewFake } from "./scripts/review-fake";

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

/*
 * `--mode review` builds docz-review's page (DESIGN-0024 section 10):
 * review.html is the second input, emitted to dist-review/ with a
 * manifest, which the review server's build turns into an import module
 * so `bun build --compile` embeds every asset. The site's dist/ and its
 * bundle budget never see it.
 */
const REVIEW = "review";

/*
 * The dev server only (a build bundles everything). The design system
 * ships TypeScript source, which Vite's optimizer does not pre-bundle,
 * and Vite does not discover the dependencies of a file under
 * node_modules. So Base UI reaches the browser as its own ESM, and its
 * store (under every dialog, tabs, popover, and menu) imports
 * use-sync-external-store's shim, which is CommonJS and has no named
 * export until it is pre-bundled. Without these two the page is blank
 * with "does not provide an export named 'useSyncExternalStore'".
 */
const DEV_PREBUNDLE = [
  "@base-ui/react > use-sync-external-store/shim",
  "@base-ui/react > use-sync-external-store/shim/with-selector",
];

export default defineConfig(({ mode }) => ({
  plugins:
    mode === REVIEW
      ? [react(), tailwindcss(), reviewFake()]
      : [react(), tailwindcss()],
  build:
    mode === REVIEW
      ? {
          target: BROWSER_FLOOR,
          outDir: "dist-review",
          manifest: true,
          rollupOptions: {
            input: fileURLToPath(new URL("./review.html", import.meta.url)),
          },
        }
      : { target: BROWSER_FLOOR },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  optimizeDeps: { include: DEV_PREBUNDLE },
  server: {
    // src/theme/tokens.css imports the themes from ../pkg/theme
    // (DESIGN-0025 §3). Setting allow replaces Vite's default, so the
    // workspace root is listed again beside it.
    fs: {
      allow: [
        searchForWorkspaceRoot(process.cwd()),
        fileURLToPath(new URL("../pkg/theme", import.meta.url)),
      ],
    },
    proxy: {
      "/api": doczApiUrl,
      "/auth": doczApiUrl,
      "/openapi.yaml": doczApiUrl,
    },
  },
}));
