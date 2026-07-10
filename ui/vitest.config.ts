import { defineConfig } from "vitest/config";

/**
 * Vitest config for rfc-site.
 *
 * - `resolve.dedupe` keeps React singleton — guards against any future
 *   nested-resolution path mounting two React instances and crashing
 *   jsdom with "Cannot read properties of null (reading 'useState')".
 * - `testTimeout: 15000` covers Shiki's WASM regex cold-start (markdown
 *   pipeline tests can exceed the 5s default on the GitHub Actions runner).
 */
export default defineConfig({
  resolve: {
    dedupe: ["react", "react-dom"],
  },
  test: {
    environment: "jsdom",
    globals: false,
    setupFiles: ["./tests/setup.ts"],
    include: ["tests/**/*.test.{ts,tsx}", "src/**/*.test.{ts,tsx}"],
    testTimeout: 15000,
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/**/index.ts"],
    },
  },
});
