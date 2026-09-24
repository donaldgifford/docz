import { setupServer } from "msw/node";

import { getDoczApiMock } from "@/api/__generated__/docz-api.msw";
import { demoOrgHandlers } from "@/mocks/fixtures";

// One MSW node server for the whole suite: curated demo-org fixtures
// (real docz markdown) first, generated faker handlers as the
// fallback for everything else. Tests override per-case with
// `server.use(...)`; setup.ts resets between tests.
export const server = setupServer(...demoOrgHandlers, ...getDoczApiMock());
