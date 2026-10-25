/// <reference types="node" />
import { readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  ContractError,
  MAX_OUTPUT_BYTES,
  VersionError,
  checkSchemaVersion,
  compareVersions,
  parseCreate,
  parseInspect,
  parseReview,
  parseValidate,
} from "@/cli/validate";

// The Go contract test's committed outputs: the same bytes docz produced.
// Resolved from the working directory, since import.meta.url is not a
// file: URL under jsdom.
const fixtureDir = resolve(process.cwd(), "../cmd/testdata/cli-contract/v1");

const parsers: Record<string, (raw: string) => unknown> = {
  inspect: parseInspect,
  validate: parseValidate,
  create: parseCreate,
  review: parseReview,
};

function fixture(name: string): string {
  return readFileSync(join(fixtureDir, name), "utf8");
}

describe("parse functions", () => {
  const files = readdirSync(fixtureDir).filter((f) => !f.startsWith("list"));

  it.each(files)("accepts the committed fixture %s", (file) => {
    const kind = (file.split("-")[0] ?? "").replace(".json", "");
    const parser = parsers[kind];
    if (parser === undefined) {
      throw new Error(`no parser for ${file}`);
    }
    expect(() => parser(fixture(file))).not.toThrow();
  });

  it("accepts an unknown field a newer docz adds", () => {
    const value = JSON.parse(fixture("inspect-design.json")) as Record<
      string,
      unknown
    >;
    value.typed_fields = { anything: true };
    expect(() => parseInspect(JSON.stringify(value))).not.toThrow();
  });

  it("refuses a missing required field", () => {
    const value = JSON.parse(fixture("inspect-design.json")) as Record<
      string,
      unknown
    >;
    delete value.content_sha256;
    expect(() => parseInspect(JSON.stringify(value))).toThrow(ContractError);
  });

  it("refuses output over the size limit", () => {
    const big = `"${"x".repeat(MAX_OUTPUT_BYTES)}"`;
    expect(() => parseCreate(big)).toThrow(/larger than/);
  });

  it("refuses something that is not JSON", () => {
    expect(() => parseValidate("Created DESIGN: x")).toThrow(/not JSON/);
  });

  it("refuses a different schema_version, naming both versions", () => {
    const value = JSON.parse(fixture("inspect-design.json")) as Record<
      string,
      unknown
    >;
    value.schema_version = 2;
    value.docz_version = "3.0.0";
    expect(() => parseInspect(JSON.stringify(value))).toThrow(
      /schema_version 2 \(docz 3\.0\.0\).*schema_version 1 from docz 2\.2\.0/,
    );
    expect(() => {
      checkSchemaVersion(2, "3.0.0");
    }).toThrow(VersionError);
    expect(() => {
      checkSchemaVersion(2, "3.0.0");
    }).toThrow(
      /schema_version 2 \(docz 3\.0\.0\).*schema_version 1 from docz 2\.2\.0/,
    );
  });

  it("refuses a docz older than the contract and accepts dev and newer", () => {
    expect(() => {
      checkSchemaVersion(1, "2.1.3");
    }).toThrow(/docz 2\.1\.3 predates.*2\.2\.0/);
    expect(() => {
      checkSchemaVersion(1, "dev");
    }).not.toThrow();
    expect(() => {
      checkSchemaVersion(1, "v2.3.0-beta.1");
    }).not.toThrow();
  });

  it("accepts a build from a checkout past an older tag", () => {
    expect(() => {
      checkSchemaVersion(1, "v2.1.3-50-g6115270");
    }).not.toThrow();
    expect(() => {
      checkSchemaVersion(1, "v2.1.3-50-g6115270-dirty");
    }).not.toThrow();
    expect(() => {
      checkSchemaVersion(2, "v2.1.3-50-g6115270");
    }).toThrow(/schema_version 2/);
  });
});

describe("compareVersions", () => {
  it("orders by major, minor, patch", () => {
    expect(compareVersions("2.2.0", "2.10.0")).toBeLessThan(0);
    expect(compareVersions("v3.0.0", "2.99.99")).toBeGreaterThan(0);
    expect(compareVersions("2.2.0", "2.2.0")).toBe(0);
  });
});
