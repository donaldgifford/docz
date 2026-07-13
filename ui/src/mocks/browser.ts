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

// e2e hook: a rendering-pipeline fixture (alert + code chrome + a
// mermaid diagram with a hostile label) served only under its flag,
// so the demo-org doc counts every other suite asserts stay stable.
const RENDERING_DOC_FLAG = "docz:e2e:rendering-doc";
const RENDERING_DOC_MD = [
  "# Rendering pipeline e2e fixture",
  "",
  "> [!WARNING]",
  "> Alert callouts render as admonitions.",
  "",
  "```go internal/ingest/parse.go",
  "func main() {}",
  "```",
  "",
  "```mermaid fig 1 - order flow",
  "flowchart TD",
  '  A["<img src=x onerror=alert(1)>"] --> B[Consumer]',
  "```",
].join("\n");
const renderingDoc = http.get(
  "*/api/v1/repos/donaldgifford/docz-site/types/design/docs/DESIGN-0777",
  () => {
    if (sessionStorage.getItem(RENDERING_DOC_FLAG) !== "1") {
      return undefined; // 404s through the fixtures
    }
    return HttpResponse.json({
      repo: "donaldgifford/docz-site",
      doc_id: "DESIGN-0777",
      type: "design",
      title: "Rendering pipeline e2e fixture",
      status: "Draft",
      author: "Donald Gifford",
      created: "2026-07-12",
      path: "docs/design/0777-rendering-pipeline-e2e-fixture.md",
      git_sha: "fixture-sha-design-0777",
      content_hash: "fixture-hash-design-0777",
      updated_at: "2026-07-12T00:00:00Z",
      raw_md: RENDERING_DOC_MD,
    });
  },
);

// Browser-side MSW worker for `just dev-msw` and the e2e build —
// curated demo-org fixtures (real docz markdown) first, generated
// faker handlers as the fallback, no docz-api needed. Loaded
// dynamically from main.tsx only when VITE_API_MODE=msw, so none of
// this reaches the production bundle.
export const worker = setupWorker(
  force401,
  renderingDoc,
  ...demoOrgHandlers,
  ...getDoczApiMock(),
);
