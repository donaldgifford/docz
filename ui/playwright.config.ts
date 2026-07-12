import { defineConfig, devices } from "@playwright/test";

/*
 * e2e against a production build with the MSW browser worker enabled
 * (IMPL-0001 Phase 4): `build:msw` bakes VITE_API_MODE=msw into a
 * separate dist-msw/ (the deployable dist/ stays MSW-free), preview
 * serves it, and the specs drive the same curated fixtures the unit
 * tests use.
 */
export default defineConfig({
  testDir: "e2e",
  fullyParallel: true,
  forbidOnly: process.env.CI !== undefined,
  retries: process.env.CI === undefined ? 0 : 2,
  reporter: process.env.CI === undefined ? "list" : "github",
  use: {
    baseURL: "http://localhost:4173",
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "bun run build:msw && bun run preview:msw",
    url: "http://localhost:4173",
    reuseExistingServer: process.env.CI === undefined,
    timeout: 120_000,
  },
});
