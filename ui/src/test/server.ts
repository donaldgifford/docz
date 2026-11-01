import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

import { getDoczApiMock } from "@/api/__generated__/docz-api.msw";
import { demoOrgHandlers } from "@/mocks/fixtures";

// One MSW node server for the whole suite: curated demo-org fixtures
// (real docz markdown) first, generated faker handlers as the
// fallback for everything else. Tests override per-case with
// `server.use(...)`; setup.ts resets between tests.
export const server = setupServer(...demoOrgHandlers, ...getDoczApiMock());

/**
 * failDocFragments makes every document's docz-html fragment a 500, so a
 * test reaches the reader's own markdown render, the fallback for a
 * fragment that does not load (DESIGN-0026 §7).
 */
export function failDocFragments(): void {
  server.use(
    http.get("*/api/v1/repos/:owner/:name/types/:type/docs/:docId/html", () =>
      HttpResponse.json({ error: "boom" }, { status: 500 }),
    ),
  );
}
