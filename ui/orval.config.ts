import { defineConfig } from "orval";

// Generates the typed TanStack Query client (and MSW mock handlers)
// from the vendored docz-api spec. Output is gitignored — regenerate
// with `bun run gen-api`; CI guards drift via `bun run gen-api:check`.
export default defineConfig({
  doczApi: {
    input: {
      target: "./api/openapi.yaml",
    },
    output: {
      target: "./src/api/__generated__/docz-api.ts",
      mode: "split",
      client: "react-query",
      httpClient: "fetch",
      clean: true,
      mock: true,
      override: {
        mutator: {
          path: "./src/api/fetcher.ts",
          name: "fetcher",
        },
      },
    },
  },
});
