import inspectJSON from "@/review/testdata/design-0042.inspect.json";

import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { Finding } from "@/cli/__generated__/validate";
import type { ReviewDocument } from "@/review/api";

/*
 * A small design document as real `docz inspect --format json` printed
 * it (testdata/design-0042.inspect.json, captured from a scratch
 * repository), shared by the review page's tests. Two open questions
 * with recommendations, one resolved, a goals region, and three of the
 * findings `docz validate` reported for it.
 */

export const INSPECT = inspectJSON as DoczInspect;
export const SOURCE = INSPECT.content;

export const FINDINGS: Finding[] = [
  {
    code: "region.missing",
    severity: "error",
    line: 0,
    kind: "api-changes",
    detail: 'the schema requires a "api-changes" region',
  },
  {
    code: "content.not-bullets",
    severity: "warning",
    line: 11,
    detail: "section is prose where the kind is a list of bullets",
  },
  {
    code: "design.goals.empty",
    severity: "warning",
    line: 12,
    kind: "goals",
    detail: "the goals section has no goals in it",
  },
];

export const TOKEN = "t".repeat(64);

export function reviewDocument(
  over: Partial<ReviewDocument> = {},
): ReviewDocument {
  return {
    key: INSPECT.id,
    round: "round-DESIGN-0042-1",
    round_number: 1,
    state: "open",
    inspect: INSPECT,
    findings: FINDINGS,
    previous: null,
    draft: null,
    ...over,
  };
}
