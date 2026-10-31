import { describe, expect, it } from "vitest";

import {
  actionableItems,
  canApprove,
  emptyDraft,
  inDocumentOrder,
  isAnswered,
  isEmptyDraft,
  outcomeOf,
  toRecordParts,
  type AnnotationDraft,
  type Draft,
} from "@/review/draft";

import type { SelectionAnchor } from "@/review/anchor";

const at = (line: number, exact = "x"): SelectionAnchor => ({
  region: "goals",
  ordinal: 0,
  heading: "Goals",
  exact,
  prefix: "",
  suffix: "",
  line,
  selection: exact,
  multiBlock: false,
});

const note = (
  id: string,
  kind: AnnotationDraft["kind"],
  line?: number,
): AnnotationDraft => ({
  id,
  kind,
  ...(line === undefined ? {} : { anchor: at(line) }),
  ...(kind === "document" || kind === "comment" ? { text: id } : {}),
});

const exact = () => ({ resolution: "exact", line: 9, ordinal: 0 }) as const;

describe("the actionable set", () => {
  const cases: [string, (d: Draft) => void][] = [
    ["a question answer", (d) => (d.questions[1] = { choice: "a" })],
    ["a comment", (d) => d.annotations.push(note("c", "comment", 3))],
    ["a replace", (d) => d.annotations.push(note("r", "replace", 3))],
    ["a delete", (d) => d.annotations.push(note("d", "delete", 3))],
    ["a document comment", (d) => d.annotations.push(note("doc", "document"))],
    ["a status change", (d) => (d.status = "Approved")],
    [
      "a finding to fix",
      (d) =>
        d.findings.push({ code: "frontmatter.status", line: 3, action: "fix" }),
    ],
  ];

  it.each(cases)("%s is actionable", (_name, edit) => {
    const d = emptyDraft("Draft");
    edit(d);
    expect(outcomeOf(d, "Draft")).toBe("changes");
    expect(canApprove(d, "Draft")).toBe(false);
  });

  it("looks good and a dismissed finding are not", () => {
    const d = emptyDraft("Draft");
    d.annotations.push(note("ok", "looks-good", 4));
    d.findings.push({ code: "references.no-link", line: 9, action: "dismiss" });
    expect(actionableItems(d, "Draft")).toEqual([]);
    expect(outcomeOf(d, "Draft")).toBe("approved");
    expect(canApprove(d, "Draft")).toBe(true);
  });

  it("only a draft with nothing in it is empty", () => {
    const d = emptyDraft("Draft");
    d.questions[1] = { note: "thinking" };
    expect(isEmptyDraft(d, "Draft")).toBe(true);
    d.annotations.push(note("ok", "looks-good", 4));
    expect(isEmptyDraft(d, "Draft")).toBe(false);
    const f = emptyDraft("Draft");
    f.findings.push({ code: "references.no-link", line: 9, action: "dismiss" });
    expect(isEmptyDraft(f, "Draft")).toBe(false);
    expect(isEmptyDraft(emptyDraft("Draft"), "Approved")).toBe(false);
  });

  it("an unanswered card and empty Other text are not answers", () => {
    expect(isAnswered(undefined)).toBe(false);
    expect(isAnswered({ note: "thinking" })).toBe(false);
    expect(isAnswered({ choice: "other", text: "  " })).toBe(false);
    expect(isAnswered({ choice: "other", text: "use yaml" })).toBe(true);
  });
});

describe("toRecordParts", () => {
  it("sends no question decisions for untouched questions", () => {
    const d = emptyDraft("Draft");
    d.questions[2] = { note: "no choice yet" };
    d.annotations.push(note("c", "comment", 5));
    const parts = toRecordParts(d, "Draft", exact);
    expect(parts.questions).toEqual([]);
    expect(parts.outcome).toBe("changes");
    expect(parts.annotations).toEqual([
      {
        order: 1,
        kind: "comment",
        anchor: {
          region: "goals",
          ordinal: 0,
          heading: "Goals",
          exact: "x",
          prefix: "",
          suffix: "",
          line: 9,
          resolution: "exact",
        },
        selection: "x",
        text: "c",
      },
    ]);
  });

  it("orders questions, keeps Other text and notes, and orders annotations", () => {
    const d = emptyDraft("Draft");
    d.questions[3] = { choice: "other", text: "put it in the spec" };
    d.questions[1] = { choice: "a", note: "agreed" };
    d.annotations.push(note("late", "delete", 40), note("doc", "document"));
    d.annotations.push({
      id: "r",
      kind: "replace",
      anchor: at(12),
      replacement: "everyday",
    });
    const parts = toRecordParts(d, "Draft", (a) => ({
      resolution: "ambiguous",
      line: a.line,
      ordinal: a.ordinal,
    }));
    expect(parts.questions).toEqual([
      { number: 1, choice: "a", note: "agreed" },
      { number: 3, choice: "other", text: "put it in the spec" },
    ]);
    expect(parts.annotations.map((a) => [a.order, a.kind])).toEqual([
      [1, "document"],
      [2, "replace"],
      [3, "delete"],
    ]);
    expect(parts.annotations[1]?.replacement).toBe("everyday");
    expect(parts.annotations[1]?.anchor?.resolution).toBe("ambiguous");
  });

  it("records the chosen status and every acted-on finding", () => {
    const d = emptyDraft("Draft");
    d.status = "In Review";
    d.findings.push({ code: "a.b", line: 1, action: "dismiss" });
    const parts = toRecordParts(d, "Draft", exact);
    expect(parts.status).toBe("In Review");
    expect(parts.findings).toEqual([
      { code: "a.b", line: 1, action: "dismiss" },
    ]);
  });
});

describe("inDocumentOrder", () => {
  it("puts document comments first and keeps draft order on a tie", () => {
    const order = inDocumentOrder([
      note("b", "comment", 7),
      note("a", "comment", 7),
      note("doc", "document"),
      note("first", "comment", 2),
    ]).map((a) => a.id);
    expect(order).toEqual(["doc", "first", "b", "a"]);
  });
});
