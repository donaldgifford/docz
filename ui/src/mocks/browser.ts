import { setupWorker } from "msw/browser";

import { getDoczApiMock } from "@/api/__generated__/docz-api.msw";
import { demoOrgHandlers } from "@/mocks/fixtures";

// Browser-side MSW worker for `just dev-msw` — curated demo-org
// fixtures (real docz markdown) first, generated faker handlers as the
// fallback, no docz-api needed. Loaded dynamically from main.tsx only
// when VITE_API_MODE=msw, so none of this reaches the production
// bundle.
export const worker = setupWorker(...demoOrgHandlers, ...getDoczApiMock());
