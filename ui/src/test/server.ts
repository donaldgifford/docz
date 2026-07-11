import { setupServer } from "msw/node";

import { getDoczApiMock } from "@/api/__generated__/docz-api.msw";

// One MSW node server for the whole suite, seeded with the generated
// handlers (faker-backed, matching the vendored spec). Tests override
// per-case with `server.use(...)`; setup.ts resets between tests.
export const server = setupServer(...getDoczApiMock());
