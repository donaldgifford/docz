import { describe, expect, test } from "bun:test";
import { mkdtempSync, readFileSync, realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { INSPECT } from "@/review/testing";

import {
  armFromHook,
  candidates,
  createdPaths,
  type HookRequest,
  reviewArmArgs,
} from "./arm";
import { Docz } from "./docz";
import { Session } from "./state";
import { fakeDocz } from "./testdata/fake-docz";

const fixture = (name: string): HookRequest =>
  JSON.parse(
    readFileSync(
      join(import.meta.dir, "testdata", "hook", `${name}.json`),
      "utf8",
    ),
  ) as HookRequest;

const inspectOf = (id: string, type: string, path: string) => ({
  stdout: JSON.stringify({ ...INSPECT, id, type, path }),
});

/** A request from the fixture with its cwd pointed at a real directory. */
function setup(name: string) {
  const cwd = realpathSync(mkdtempSync(join(tmpdir(), "arm-repo-")));
  const base = mkdtempSync(join(tmpdir(), "arm-state-"));
  const req = fixture(name);
  req.cwd = cwd;
  const fake = fakeDocz({
    "inspect docs/design/0001-review-cache.md": [
      inspectOf("DESIGN-0001", "design", "docs/design/0001-review-cache.md"),
    ],
    "inspect DESIGN-0001": [
      inspectOf("DESIGN-0001", "design", "docs/design/0001-review-cache.md"),
    ],
    "inspect /work/repo/docs/adr/0001-use-valkey.md": [
      inspectOf("ADR-0001", "adr", "docs/adr/0001-use-valkey.md"),
    ],
    "inspect docs/adr/0001-use-valkey.md": [
      inspectOf("ADR-0001", "adr", "docs/adr/0001-use-valkey.md"),
    ],
    "inspect /work/repo/docs/rfc/0001-long.md": [
      inspectOf("RFC-0001", "rfc", "docs/rfc/0001-long.md"),
    ],
    "inspect /work/repo/docs/design/0002-a.md": [
      inspectOf("DESIGN-0002", "design", "docs/design/0002-a.md"),
    ],
    inspect: [{ code: 1, stderr: "Error: no document\n" }],
  });
  const run = () =>
    armFromHook(JSON.stringify(req), { docz: new Docz(fake.bin), base });
  const session = () =>
    new Session({ base, root: cwd, session: req.session_id as string });
  return { cwd, run, session, fake };
}

describe("candidates", () => {
  test("reads the JSON envelope docz create printed", () => {
    expect(candidates(fixture("create-json"))).toEqual([
      "docs/design/0001-review-cache.md",
    ]);
  });

  test("reads Created lines when there is no envelope", () => {
    expect(candidates(fixture("create-text"))).toEqual([
      "/work/repo/docs/adr/0001-use-valkey.md",
    ]);
    expect(candidates(fixture("compound"))).toEqual([
      "/work/repo/docs/design/0002-a.md",
      "/work/repo/docs/adr/0002-b.md",
    ]);
  });

  test("finds the Created line in long, truncated output", () => {
    expect(candidates(fixture("long-output"))).toEqual([
      "/work/repo/docs/rfc/0001-long.md",
    ]);
  });

  test("an echo of the line arms nothing: no docz word in the command", () => {
    expect(candidates(fixture("echo"))).toEqual([]);
  });

  test("a failed create and an unrelated command have no candidates", () => {
    expect(candidates(fixture("create-failure"))).toEqual([]);
    expect(candidates(fixture("other"))).toEqual([]);
  });

  test("reads docz-review arm's arguments from the command", () => {
    expect(candidates(fixture("review-arm"))).toEqual([
      "DESIGN-0001",
      "docs/adr/0001-use-valkey.md",
    ]);
    expect(
      reviewArmArgs("cd x && /opt/bin/docz-review arm A --json B; ls"),
    ).toEqual(["A", "B"]);
  });

  test("only a docz envelope counts as one", () => {
    expect(
      createdPaths('{\n  "path": "x"\n}\nCreated RFC: docs/rfc/1.md\n'),
    ).toEqual(["docs/rfc/1.md"]);
  });
});

describe("armFromHook", () => {
  test("records a created document as pending under the session", async () => {
    const { run, session, cwd } = setup("create-json");
    const res = await run();
    expect(res.stdout).toBe("");
    expect(res.armed).toHaveLength(1);
    expect(session().read().entries[0]).toMatchObject({
      id: "DESIGN-0001",
      path: "docs/design/0001-review-cache.md",
      root: cwd,
      state: "pending",
      round: 0,
    });
  });

  test("validates through docz inspect with --repo-root <cwd>", async () => {
    const { run, fake, cwd } = setup("create-text");
    await run();
    expect(fake.calls()[0]).toEqual({
      args: [
        "inspect",
        "/work/repo/docs/adr/0001-use-valkey.md",
        "--format",
        "json",
        "--repo-root",
        cwd,
      ],
      cwd,
    });
  });

  test("a candidate docz cannot resolve is dropped with a systemMessage", async () => {
    const { run, session } = setup("compound");
    const res = await run();
    expect(res.armed.map((e) => e.id)).toEqual(["DESIGN-0002"]);
    expect(res.dropped.map((d) => d.candidate)).toEqual([
      "/work/repo/docs/adr/0002-b.md",
    ]);
    const msg = JSON.parse(res.stdout) as { systemMessage: string };
    expect(msg.systemMessage).toContain(
      "not armed: /work/repo/docs/adr/0002-b.md",
    );
    expect(session().read().entries).toHaveLength(1);
  });

  test("the skill's explicit arm records every named document", async () => {
    const { run } = setup("review-arm");
    expect((await run()).armed.map((e) => e.id)).toEqual([
      "DESIGN-0001",
      "ADR-0001",
    ]);
  });

  test("other commands, echoes, and failures print nothing and run nothing", async () => {
    for (const name of ["other", "echo", "create-failure"]) {
      const { run, fake } = setup(name);
      expect(await run()).toEqual({ armed: [], dropped: [], stdout: "" });
      expect(fake.calls()).toEqual([]);
    }
  });

  test("re-arming an approved document returns it to pending", async () => {
    const { run, session } = setup("create-json");
    const [entry] = (await run()).armed;
    const s = session();
    s.apply(entry?.key ?? "", "open");
    s.apply(entry?.key ?? "", "approved");
    await run();
    expect(s.read().entries[0]).toMatchObject({ state: "pending", round: 1 });
  });

  test("a request that is not JSON is ignored", async () => {
    const fake = fakeDocz({});
    const res = await armFromHook("not json", {
      docz: new Docz(fake.bin),
      base: mkdtempSync(join(tmpdir(), "arm-state-")),
    });
    expect(res.stdout).toBe("");
  });
});
