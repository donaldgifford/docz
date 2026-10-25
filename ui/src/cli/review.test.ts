/// <reference types="node" />
import { readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

import { Ajv2020 } from "ajv/dist/2020";
import { describe, expect, it } from "vitest";

import { ContractError, parseReview, VersionError } from "@/cli/validate";

/*
 * The review record's contract test (DESIGN-0024 section 8). docz-review
 * is the record's only producer today, so the committed fixtures are the
 * contract: each must satisfy the closed producer schema and parse
 * through the open consumer variant docz-review itself uses. The Go
 * contract test re-validates the same files with formats asserted.
 */

const fixtureDir = resolve(process.cwd(), "../cmd/testdata/cli-contract/v1");
const closedSchema = JSON.parse(
  readFileSync(resolve(process.cwd(), "../api/cli/review.schema.json"), "utf8"),
) as object;

const ajv = new Ajv2020({ allErrors: true, validateFormats: false });
ajv.addKeyword("$version");
const closed = ajv.compile(closedSchema);

const records = readdirSync(fixtureDir).filter((f) => f.startsWith("review-"));
const read = (f: string) => readFileSync(join(fixtureDir, f), "utf8");
const everyKind = () =>
  JSON.parse(read("review-every-kind.json")) as Record<string, unknown>;

describe("the review record", () => {
  it("has committed fixtures for every outcome", () => {
    const outcomes = records.map(
      (f) => (JSON.parse(read(f)) as { outcome: string }).outcome,
    );
    expect(new Set(outcomes)).toEqual(
      new Set(["approved", "changes", "dismissed"]),
    );
  });

  it.each(records)("%s satisfies the closed schema and parses", (file) => {
    const raw = read(file);
    expect(closed(JSON.parse(raw)), ajv.errorsText(closed.errors)).toBe(true);
    expect(parseReview(raw).schema_version).toBe(1);
  });

  it("covers every annotation kind and resolution", () => {
    const rec = parseReview(read("review-every-kind.json"));
    expect(new Set(rec.annotations.map((a) => a.kind))).toEqual(
      new Set(["comment", "replace", "delete", "document", "looks-good"]),
    );
    expect(
      new Set(rec.annotations.flatMap((a) => a.anchor?.resolution ?? [])),
    ).toEqual(new Set(["exact", "ambiguous", "lost"]));
  });

  it("an unknown field fails the producer schema and passes the consumer", () => {
    const rec = { ...everyKind(), reviewer: "someone" };
    expect(closed(rec)).toBe(false);
    expect(parseReview(JSON.stringify(rec)).outcome).toBe("changes");
  });

  it.each([
    ["a missing outcome", (r: Record<string, unknown>) => delete r.outcome],
    [
      "an unknown annotation kind",
      (r: Record<string, unknown>) => {
        (r.annotations as { kind: string }[])[0] = { kind: "shout" };
      },
    ],
    [
      "a digest that is not hex SHA-256",
      (r: Record<string, unknown>) => {
        (r.document as { content_sha256: string }).content_sha256 = "abc";
      },
    ],
    [
      "an option that is not a letter or other",
      (r: Record<string, unknown>) => {
        (r.questions as { choice: string }[])[0] = { choice: "A" };
      },
    ],
  ])("%s is refused", (_, mutate) => {
    const rec = everyKind();
    mutate(rec);
    expect(() => parseReview(JSON.stringify(rec))).toThrow(ContractError);
  });

  it("another schema_version is refused naming both", () => {
    const rec = { ...everyKind(), schema_version: 2 };
    expect(() => parseReview(JSON.stringify(rec))).toThrow(VersionError);
    expect(() => parseReview(JSON.stringify(rec))).toThrow(
      /schema_version 2.*schema_version 1/,
    );
  });
});
