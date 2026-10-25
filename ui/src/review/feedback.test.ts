import { describe, expect, it } from "vitest";

import { feedbackMessage, type FeedbackDocument } from "@/review/feedback";

import type { Question } from "@/cli/__generated__/inspect";
import type { DoczReview } from "@/cli/__generated__/review";

// Goldens under testdata/feedback; regenerate with `bunx vitest -u`.
const golden = (name: string) => `./testdata/feedback/${name}.txt`;

const question = (
  number: number,
  title: string,
  resolved = false,
): Question => ({
  number,
  title,
  line: 400 + number,
  options: [],
  resolved: resolved
    ? { date: "2026-10-01", choice: "a", note: "", line: 410 + number }
    : null,
});

const record = (over: Partial<DoczReview>): DoczReview => ({
  schema_version: 1,
  review_id: "r_3f9a",
  round: 2,
  submission_id: "s_17c2",
  submitted_at: "2026-10-24T15:04:05Z",
  document: {
    id: "DESIGN-0024",
    path: "docs/design/0024-docz-review.md",
    type: "design",
    content_sha256: "9f3c",
  },
  outcome: "changes",
  questions: [],
  annotations: [],
  status: "Draft",
  findings: [],
  ...over,
});

const anchor = (
  region: string,
  heading: string,
  exact: string,
  line: number,
  resolution: "exact" | "ambiguous" | "lost" = "exact",
) => ({
  region,
  ordinal: 0,
  heading,
  exact,
  prefix: "",
  suffix: "",
  line,
  resolution,
});

// The design's own example, section 8.
const DESIGN_EXAMPLE: FeedbackDocument = {
  title: "docz-review…",
  currentStatus: "Draft",
  questions: [
    question(1, "Where do the schemas live?"),
    question(2, "Already settled?", true),
    question(3, "How are the TypeScript types produced?"),
    question(5, "What is the round limit?"),
  ],
  findings: [
    { code: "references.no-link", line: 589, severity: "warning" },
    { code: "frontmatter.status", line: 3, severity: "error" },
  ],
  record: record({
    questions: [
      { number: 1, choice: "a" },
      {
        number: 3,
        choice: "other",
        text: "put the shapes in api/openapi.yaml",
      },
    ],
    annotations: [
      {
        order: 1,
        kind: "comment",
        anchor: anchor(
          "detailed-design",
          "3. docz inspect",
          "Anchors are quotes.",
          212,
        ),
        selection: "Anchors are quotes.",
        text: "Say what INV-0021 decided once it closes.",
      },
      {
        order: 2,
        kind: "delete",
        anchor: anchor("open-questions", "4.", "option (c)", 401),
        selection: "option (c)",
      },
      {
        order: 3,
        kind: "replace",
        anchor: anchor("background", "", "day-to-day", 96, "lost"),
        selection: "day-to-day",
        replacement: "everyday",
      },
      { order: 4, kind: "document", text: "Shorten the Background section." },
      {
        order: 5,
        kind: "looks-good",
        anchor: anchor("overview", "Overview", "fine", 50),
        selection: "fine",
      },
    ],
    status: "In Review",
    findings: [
      { code: "references.no-link", line: 589, action: "dismiss" },
      { code: "frontmatter.status", line: 3, action: "fix" },
    ],
  }),
};

describe("feedbackMessage", () => {
  it("writes the design's example", async () => {
    await expect(feedbackMessage([DESIGN_EXAMPLE])).toMatchFileSnapshot(
      golden("design-example"),
    );
  });

  it("names only the answered questions in the closing instruction", () => {
    const text = feedbackMessage([DESIGN_EXAMPLE]);
    expect(text).toContain("Resolve Q1 and Q3 in the document");
    expect(text).not.toContain("Q5 in the document");
    expect(text).toContain("- Q5 (no answer; leave open)");
    expect(text).not.toContain("Q2");
  });

  it("writes a status-only round and an approved document", async () => {
    const statusOnly: FeedbackDocument = {
      title: "A plan",
      currentStatus: "Draft",
      questions: [question(1, "Pick one?")],
      findings: [],
      record: record({
        document: {
          id: "IMPL-0003",
          path: "docs/impl/0003-a-plan.md",
          type: "impl",
          content_sha256: "ab",
        },
        round: 1,
        status: "In Progress",
      }),
    };
    const approved: FeedbackDocument = {
      title: "An approved record",
      currentStatus: "Proposed",
      questions: [],
      findings: [{ code: "references.no-link", line: 9, severity: "warning" }],
      record: record({
        document: {
          id: "ADR-0006",
          path: "docs/adr/0006-x.md",
          type: "adr",
          content_sha256: "cd",
        },
        round: 1,
        outcome: "approved",
        status: "Proposed",
        findings: [{ code: "references.no-link", line: 9, action: "dismiss" }],
      }),
    };
    await expect(feedbackMessage([statusOnly, approved])).toMatchFileSnapshot(
      golden("status-and-approved"),
    );
  });

  it("writes no closing instruction for an approved or dismissed document", () => {
    for (const outcome of ["approved", "dismissed"] as const) {
      const text = feedbackMessage([
        { ...DESIGN_EXAMPLE, record: record({ outcome }) },
      ]);
      expect(text).not.toContain("finish your turn");
    }
  });
});
