import { describe, expect, test } from "bun:test";
import { mkdtempSync, realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { INSPECT } from "@/review/testing";

import { Session } from "./state";
import { fakeDocz } from "./testdata/fake-docz";
import { docAs, doczConfig } from "./testdata/review-fixture";

const A = docAs(INSPECT.id, INSPECT.path);
const MAIN = join(import.meta.dir, "main.ts");

/**
 * The real process under a real signal: docz-review as the Stop hook,
 * with no browser to open, so it prints the URL to stderr and waits.
 */
async function hookUnderSignal(signal: NodeJS.Signals) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "sig-repo-")));
  const base = mkdtempSync(join(tmpdir(), "sig-state-"));
  const fake = fakeDocz(doczConfig([A]));
  const session = new Session({ base, root, session: "sig" });
  session.arm({
    id: A.inspect.id,
    type: A.inspect.type,
    path: A.inspect.path,
    root,
    config: "",
    docs_dir: "",
  });
  const proc = Bun.spawn(["bun", MAIN, "--pending", "--hook"], {
    env: {
      ...process.env,
      DOCZ_REVIEW_DOCZ: fake.bin,
      DOCZ_REVIEW_STATE_DIR: base,
      DOCZ_REVIEW_BROWSER: "/nonexistent/browser",
    },
    stdin: new Blob([JSON.stringify({ session_id: "sig", cwd: root })]),
    stdout: "pipe",
    stderr: "pipe",
  });
  // Wait for the server to be up: the URL line on stderr.
  const reader = proc.stderr.getReader();
  let err = "";
  while (!err.includes("open http://127.0.0.1:")) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    err += new TextDecoder().decode(value);
  }
  proc.kill(signal);
  const code = await proc.exited;
  return {
    code,
    out: await new Response(proc.stdout).text(),
    state: session.read().entries[0]?.state,
  };
}

describe("signals", () => {
  for (const signal of ["SIGTERM", "SIGHUP", "SIGINT"] as const) {
    test(`${signal} is a dismissal with nothing on stdout`, async () => {
      expect(await hookUnderSignal(signal)).toEqual({
        code: 0,
        out: "",
        state: "dismissed",
      });
    });
  }
});
