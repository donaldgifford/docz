import { defineConfig } from "orval";

/**
 * orval configuration — generates a typed TanStack Query client for
 * `rfc-api` from the vendored OpenAPI 3.1 spec at `api/openapi.yaml`.
 *
 * Per ADR-0001 the generated output is **not committed** — it lives at
 * `src/portal/api/__generated__/` (gitignored) and is regenerated via
 * `bun run gen-api` (or `just gen-api`). CI's drift check re-runs the
 * generator and `git diff --exit-code`s the dir; any drift fails the
 * pipeline.
 *
 * - `client: "react-query"` — generates query / mutation hooks plus
 *   the schema types in one shot.
 * - `httpClient: "fetch"` — orval emits the call sites as a thin
 *   wrapper around our custom mutator (`src/portal/api/fetcher.ts`),
 *   which itself uses the platform `fetch`. Resolved per IMPL-0001
 *   §Phase 3 open question (zero-deps over `axios` interceptors).
 * - `mock: true` — emits MSW handlers alongside the hooks; consumed by
 *   the vitest smoke test today and by `bun run dev` in `API_MODE=msw`
 *   if we ever add that fork (Oxide-style).
 */

export default defineConfig({
  rfcApi: {
    output: {
      mode: "tags-split",
      target: "src/portal/api/__generated__/index.ts",
      schemas: "src/portal/api/__generated__/model",
      client: "react-query",
      httpClient: "fetch",
      clean: true,
      mock: true,
      override: {
        mutator: {
          path: "./src/portal/api/fetcher.ts",
          name: "rfcApiFetcher",
        },
        query: {
          useQuery: true,
          useSuspenseQuery: true,
          signal: true,
        },
      },
    },
    input: {
      target: "./api/openapi.yaml",
    },
  },
});
