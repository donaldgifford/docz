import { describe, expect, test } from "bun:test";

import { ContractError, VersionError } from "@/cli/validate";
import { FINDINGS, INSPECT } from "@/review/testing";

import { Docz, DoczError, resolveDocz } from "./docz";
import { fakeDocz, validateOutput, version } from "./testdata/fake-docz";

const scope = { root: "/tmp", config: "" };

/** The error a promise rejects with; a resolution fails the test. */
const rejection = (p: Promise<unknown>): Promise<unknown> =>
  p.then(
    () => {
      throw new Error("resolved");
    },
    (err: unknown) => err,
  );
const inspectOut = (over: object = {}) => ({
  stdout: JSON.stringify({ ...INSPECT, ...over }),
});

describe("resolveDocz", () => {
  test("prefers DOCZ_REVIEW_DOCZ, then PATH", () => {
    expect(resolveDocz({ DOCZ_REVIEW_DOCZ: "/x/docz" }, () => "/p/docz")).toBe(
      "/x/docz",
    );
    expect(resolveDocz({}, () => "/p/docz")).toBe("/p/docz");
    expect(resolveDocz({ DOCZ_REVIEW_DOCZ: "bin/docz" })).toBe(
      `${process.cwd()}/bin/docz`,
    );
    expect(resolveDocz({}, () => null)).toBeNull();
  });
});

describe("Docz", () => {
  test("version is checked once against the contract", async () => {
    const fake = fakeDocz({ version: version("v2.2.1") });
    const docz = new Docz(fake.bin);
    expect(await docz.version()).toBe("v2.2.1");
    expect(await docz.version()).toBe("v2.2.1");
    expect(fake.calls()).toHaveLength(1);
  });

  test("an old docz is refused naming both versions", async () => {
    const docz = new Docz(fakeDocz({ version: version("v2.1.0") }).bin);
    const err = await rejection(docz.version());
    expect(err).toBeInstanceOf(VersionError);
    expect((err as Error).message).toMatch(/2\.1\.0.*2\.2\.0/);
  });

  test("runs from the root with the target after -- as one argument", async () => {
    const fake = fakeDocz({ inspect: [inspectOut()] });
    const root = (await import("node:fs")).realpathSync("/tmp");
    await new Docz(fake.bin).inspect("--config=x.yaml; rm -rf /", {
      root,
      config: "alt.yaml",
    });
    expect(fake.calls()[0]).toEqual({
      args: [
        "inspect",
        "--format",
        "json",
        "--repo-root",
        root,
        "--config",
        "alt.yaml",
        "--",
        "--config=x.yaml; rm -rf /",
      ],
      cwd: root,
    });
  });

  test("a non-zero inspect is an error carrying docz's message", async () => {
    const fake = fakeDocz({
      inspect: [{ code: 1, stderr: 'Error: no document "NOPE-1"\n' }],
    });
    const err = await new Docz(fake.bin)
      .inspect("NOPE-1", scope)
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(DoczError);
    expect(err).toMatchObject({ kind: "exit", exitCode: 1 });
    expect((err as Error).message).toContain('no document "NOPE-1"');
  });

  test("output that breaks the contract is a ContractError", async () => {
    const fake = fakeDocz({ inspect: [{ stdout: '{"schema_version":1}' }] });
    expect(
      await rejection(new Docz(fake.bin).inspect("X", scope)),
    ).toBeInstanceOf(ContractError);
  });

  test("a missing binary is a spawn error, not a report", async () => {
    const err = await new Docz("/nonexistent/docz")
      .inspect("X", scope)
      .catch((e: unknown) => e);
    expect(err).toMatchObject({ kind: "spawn" });
  });

  test("validate's exit 1 is findings; exit 2 with no report is an error", async () => {
    const fake = fakeDocz({
      validate: [
        validateOutput(INSPECT, FINDINGS),
        { code: 2, stderr: "Error: bad flag\n" },
      ],
    });
    const docz = new Docz(fake.bin);
    expect("docs" in (await docz.validate(INSPECT.path, scope))).toBe(true);
    expect(await rejection(docz.validate(INSPECT.path, scope))).toMatchObject({
      kind: "exit",
      exitCode: 2,
    });
  });

  test("snapshot pairs inspect and validate over the same bytes", async () => {
    const fake = fakeDocz({
      inspect: [inspectOut()],
      validate: [validateOutput(INSPECT, FINDINGS)],
    });
    const snap = await new Docz(fake.bin).snapshot("DESIGN-0001", scope);
    expect(snap.inspect.id).toBe(INSPECT.id);
    expect(snap.findings).toEqual(FINDINGS);
    // validate is asked about the path inspect resolved, not the id.
    expect(fake.calls()[1]?.args.at(-1)).toBe(INSPECT.path);
  });

  test("snapshot retries once when the digests disagree, then fails", async () => {
    const other = { ...INSPECT, content_sha256: "0".repeat(64) };
    const once = fakeDocz({
      inspect: [inspectOut(), inspectOut()],
      validate: [validateOutput(other), validateOutput(INSPECT)],
    });
    expect((await new Docz(once.bin).snapshot("D", scope)).findings).toEqual(
      [],
    );

    const always = fakeDocz({
      inspect: [inspectOut()],
      validate: [validateOutput(other)],
    });
    expect(
      await rejection(new Docz(always.bin).snapshot("D", scope)),
    ).toMatchObject({
      kind: "digest",
    });
  });
});
