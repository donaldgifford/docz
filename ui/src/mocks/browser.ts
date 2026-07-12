import { setupWorker } from "msw/browser";

import { getDoczApiMock } from "@/api/__generated__/docz-api.msw";

// Browser-side MSW worker for `just dev-msw` — the whole API surface
// served from the generated (faker-backed) handlers, no docz-api
// needed. Loaded dynamically from main.tsx only when
// VITE_API_MODE=msw, so none of this reaches the production bundle.
export const worker = setupWorker(...getDoczApiMock());
