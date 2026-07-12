import { http, HttpResponse } from "msw";
import { setupWorker } from "msw/browser";

import { getDoczApiMock } from "@/api/__generated__/docz-api.msw";
import { demoOrgHandlers } from "@/mocks/fixtures";

// e2e hook: MSW intercepts in-page before Playwright ever sees the
// request, so the 401 journey is driven by a sessionStorage flag the
// test sets instead of network-level interception.
const FORCE_401_FLAG = "docz:e2e:force-401";
const force401 = http.all("*/api/v1/*", () => {
  if (sessionStorage.getItem(FORCE_401_FLAG) === "1") {
    return HttpResponse.json({ error: "session required" }, { status: 401 });
  }
  return undefined; // fall through to the fixtures
});

// Browser-side MSW worker for `just dev-msw` and the e2e build —
// curated demo-org fixtures (real docz markdown) first, generated
// faker handlers as the fallback, no docz-api needed. Loaded
// dynamically from main.tsx only when VITE_API_MODE=msw, so none of
// this reaches the production bundle.
export const worker = setupWorker(
  force401,
  ...demoOrgHandlers,
  ...getDoczApiMock(),
);
