/*
 * The plugin's hooks, run the way Claude Code runs them: `bash` on the
 * wrapper with the request on stdin. docz-review is a shim on PATH that
 * starts ui/review/main.ts from source, and docz is the fake the
 * server's own tests use, so nothing here needs a build.
 *
 * The requests are ui/review/testdata/hook/, the fixtures the IMPL-0027
 * Phase 1 drill will replace with captures.
 */
import { describe, expect, test } from "bun:test";
import { spawnSync } from "node:child_process";
import {
  chmodSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

import { fakeDocz, version } from "../../../ui/review/testdata/fake-docz";

const PLUGIN = resolve(import.meta.dir, "..");
const UI = resolve(PLUGIN, "../../ui");
const FIXTURES = join(UI, "review/testdata/hook");
const INSPECT = JSON.parse(
  readFileSync(
    join(UI, "src/review/testdata/design-0042.inspect.json"),
    "utf8",
  ),
) as Record<string, unknown>;

/** PATH with only the system directories: no docz-review anywhere. */
const BARE_PATH = "/usr/bin:/bin";

/** A directory holding a docz-review that runs the server from source. */
function shimDir(): string {
  const dir = mkdtempSync(join(tmpdir(), "docz-review-shim-"));
  const bin = join(dir, "docz-review");
  writeFileSync(
    bin,
    `#!/bin/sh\nexec '${process.execPath}' '${join(UI, "review/main.ts")}' "$@"\n`,
  );
  chmodSync(bin, 0o755);
  return dir;
}

const inspectOf = (id: string, type: string, path: string) => ({
  stdout: JSON.stringify({ ...INSPECT, id, type, path }),
});

interface Run {
  code: number | null;
  stdout: string;
  stderr: string;
}

function hook(
  script: "arm.sh" | "stop.sh",
  input: string,
  env: Record<string, string>,
): Run {
  const r = spawnSync("/bin/bash", [join(PLUGIN, "hooks", script)], {
    input,
    env,
    encoding: "utf8",
    timeout: 30_000,
  });
  return { code: r.status, stdout: r.stdout, stderr: r.stderr };
}

/** A repository directory, a state directory, and the hook environment. */
const readFixture = (name: string) =>
  readFileSync(join(FIXTURES, `${name}.json`), "utf8");

function setup() {
  const cwd = realpathSync(mkdtempSync(join(tmpdir(), "plugin-repo-")));
  const state = mkdtempSync(join(tmpdir(), "plugin-state-"));
  const fake = fakeDocz({
    "--version": version(),
    version: version(),
    "inspect /work/repo/docs/adr/0001-use-valkey.md": [
      inspectOf("ADR-0001", "adr", "docs/adr/0001-use-valkey.md"),
    ],
    "inspect DESIGN-0001": [
      inspectOf("DESIGN-0001", "design", "docs/design/0001-review-cache.md"),
    ],
    "inspect docs/adr/0001-use-valkey.md": [
      inspectOf("ADR-0001", "adr", "docs/adr/0001-use-valkey.md"),
    ],
    inspect: [{ code: 1, stderr: "Error: no document\n" }],
  });
  const env = {
    PATH: `${shimDir()}:${dirname(process.execPath)}:${BARE_PATH}`,
    HOME: state,
    DOCZ_REVIEW_DOCZ: fake.bin,
    DOCZ_REVIEW_STATE_DIR: state,
  };
  const request = (name: string) => {
    const req = JSON.parse(
      readFileSync(join(FIXTURES, `${name}.json`), "utf8"),
    ) as Record<string, unknown>;
    req.cwd = cwd;
    return JSON.stringify(req);
  };
  /** `docz-review pending` for the repository, through the shim. */
  const pending = () =>
    spawnSync("docz-review", ["pending", "--repo-root", cwd], {
      env,
      encoding: "utf8",
    }).stdout;
  return { cwd, env, request, pending };
}

describe("without the binary", () => {
  test("arm.sh tells the person after docz create, on stdout", () => {
    const r = hook("arm.sh", readFixture("create-text"), { PATH: BARE_PATH });
    expect(r.code).toBe(0);
    const out = JSON.parse(r.stdout) as { systemMessage: string };
    expect(out.systemMessage).toContain(
      "docz-review is not installed, so this document will not be reviewed; see https://",
    );
  });

  test("arm.sh stays silent after any other command", () => {
    const r = hook("arm.sh", readFixture("other"), { PATH: BARE_PATH });
    expect(r).toMatchObject({ code: 0, stdout: "", stderr: "" });
  });

  test("stop.sh writes one stderr line and exits 0", () => {
    const r = hook("stop.sh", '{"session_id":"s","cwd":"/"}', {
      PATH: BARE_PATH,
    });
    expect(r.code).toBe(0);
    expect(r.stdout).toBe("");
    expect(r.stderr.trim().split("\n")).toHaveLength(1);
    expect(r.stderr).toContain("docz-review is not installed; see https://");
  });
});

describe("arm.sh", () => {
  test("arms the document docz create reported, silently", () => {
    const s = setup();
    const r = hook("arm.sh", s.request("create-text"), s.env);
    expect(r).toMatchObject({ code: 0, stdout: "" });
    expect(s.pending()).toContain("ADR-0001\tpending\tround 0");
  });

  test("arms what docz-review arm named", () => {
    const s = setup();
    const r = hook("arm.sh", s.request("review-arm"), s.env);
    expect(r).toMatchObject({ code: 0, stdout: "" });
    const out = s.pending();
    expect(out).toContain("DESIGN-0001\tpending");
    expect(out).toContain("ADR-0001\tpending");
  });

  test("names a candidate docz cannot resolve in a systemMessage", () => {
    const s = setup();
    const r = hook("arm.sh", s.request("compound"), s.env);
    expect(r.code).toBe(0);
    const msg = JSON.parse(r.stdout) as { systemMessage: string };
    expect(msg.systemMessage).toContain("not armed");
    expect(msg.systemMessage).toContain("0002-a.md");
  });

  for (const name of ["create-failure", "echo", "other"]) {
    test(`${name}: prints nothing and arms nothing`, () => {
      const s = setup();
      const r = hook("arm.sh", s.request(name), s.env);
      expect(r).toMatchObject({ code: 0, stdout: "" });
      expect(s.pending()).toContain("no documents under review");
    });
  }
});

describe("stop.sh", () => {
  test("lets the turn end when nothing is armed", () => {
    const s = setup();
    const r = hook(
      "stop.sh",
      JSON.stringify({
        session_id: "9f1c2a4e-0b7d-4f7e-9a51-3c8e2d6b1f00",
        cwd: s.cwd,
        hook_event_name: "Stop",
        stop_hook_active: false,
      }),
      s.env,
    );
    expect(r).toMatchObject({ code: 0, stdout: "" });
  });
});
