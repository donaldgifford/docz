import { describe, expect, test } from "bun:test";
import { mkdtempSync, realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { parseReview } from "@/cli/validate";
import { INSPECT } from "@/review/testing";

import { Lease } from "./lease";
import { type Io, main } from "./main";
import type { AssetSource } from "./server";
import { Session } from "./state";
import { type Act, fakeBrowser, keyOf } from "./testdata/browser";
import { fakeDocz } from "./testdata/fake-docz";
import { docAs, doczConfig } from "./testdata/review-fixture";

const A = docAs(INSPECT.id, INSPECT.path);
const B = docAs("DESIGN-0043", "docs/design/0043-b.md");
const C = docAs("DESIGN-0044", "docs/design/0044-c.md");
const SESSION = "9f1c2a4e-0b7d-4f7e-9a51-3c8e2d6b1f00";

const changes = {
  outcome: "changes",
  questions: [{ number: 1, choice: "b" }],
  annotations: [
    { order: 1, kind: "document", text: "Shorten the background." },
  ],
  status: "",
  findings: [],
};

const noAssets: AssetSource = () => Promise.resolve(null);
const stream = (s: string) => new Blob([s]).stream();

function world(docs = [A, B, C]) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "main-repo-")));
  const base = mkdtempSync(join(tmpdir(), "main-state-"));
  const fake = fakeDocz(doczConfig(docs));
  const env = {
    DOCZ_REVIEW_DOCZ: fake.bin,
    DOCZ_REVIEW_STATE_DIR: base,
    PATH: "",
  };
  const run = async (
    argv: string[],
    over: Partial<Io> = {},
  ): Promise<{ code: number; out: string; err: string }> => {
    let out = "";
    let err = "";
    const code = await main({
      argv,
      env,
      cwd: root,
      stdin: stream(""),
      out: (s) => {
        out += s;
      },
      err: (s) => {
        err += s;
      },
      openBrowser: () => Promise.resolve(false),
      assets: noAssets,
      onSignal: () => () => undefined,
      ...over,
    });
    return { code, out, err };
  };
  const hookArm = (cmd: string, stdout: string) =>
    run(["arm", "--from-hook"], {
      stdin: stream(
        JSON.stringify({
          session_id: SESSION,
          cwd: root,
          tool_name: "Bash",
          tool_input: { command: cmd },
          tool_response: {
            stdout,
            stderr: "",
            interrupted: false,
            isImage: false,
          },
        }),
      ),
    });
  const stop = (act: Act, over: Partial<Io> = {}) =>
    run(["--pending", "--hook", "--grace", "5"], {
      stdin: stream(
        JSON.stringify({
          session_id: SESSION,
          cwd: root,
          hook_event_name: "Stop",
        }),
      ),
      openBrowser: fakeBrowser(act),
      ...over,
    });
  const session = () => new Session({ base, root, session: SESSION });
  return { root, base, fake, env, run, hookArm, stop, session };
}

const created = (...docs: (typeof A)[]) =>
  docs.map((d) => `Created DESIGN: ${d.inspect.path}\n`).join("");

describe("the Stop hook", () => {
  test("three documents: approve A, changes on B, C untouched", async () => {
    const w = world();
    expect(
      (await w.hookArm('docz create design "x"', created(A, B, C))).out,
    ).toBe("");
    expect(
      w
        .session()
        .read()
        .entries.map((e) => e.state),
    ).toEqual(["pending", "pending", "pending"]);
    const res = await w.stop(async (s, post) => {
      await post(`/api/review/${A.inspect.id}/approve`, keyOf(s, 0, "s_a"));
      await post(`/api/review/${B.inspect.id}/submit`, {
        ...keyOf(s, 1, "s_b"),
        ...changes,
      });
      await post("/api/review/done", {});
    });
    expect(res.code).toBe(0);
    const lines = res.out.trim().split("\n");
    expect(lines).toHaveLength(1);
    const out = JSON.parse(lines[0] ?? "") as {
      decision: string;
      reason: string;
    };
    expect(out.decision).toBe("block");
    expect(out.reason).toContain("DESIGN-0043");
    expect(out.reason).not.toContain("DESIGN-0042");
    expect(out.reason).not.toContain("DESIGN-0044");
    // Done dismisses what was never visited.
    expect(
      w
        .session()
        .read()
        .entries.map((e) => e.state),
    ).toEqual(["approved", "changes", "dismissed"]);
  });

  test("a document in changes is the next Stop's round 2", async () => {
    const w = world([A]);
    await w.hookArm("docz create design x", created(A));
    await w.stop(async (s, post) => {
      await post(`/api/review/${A.inspect.id}/submit`, {
        ...keyOf(s, 0, "s_1"),
        ...changes,
      });
    });
    let round = 0;
    const res = await w.stop(async (s, post) => {
      round = s.documents[0]?.round_number ?? 0;
      await post(`/api/review/${A.inspect.id}/approve`, keyOf(s, 0, "s_2"));
    });
    expect(round).toBe(2);
    expect(res.out).toBe("");
    expect(w.session().read().entries[0]?.state).toBe("approved");
  });

  test("nothing armed means nothing printed and no server", async () => {
    const w = world();
    let opened = false;
    const res = await w.stop(() => {
      opened = true;
      return Promise.resolve();
    });
    expect(res).toEqual({ code: 0, out: "", err: "" });
    expect(opened).toBe(false);
  });

  test("a signal is a dismissal and prints nothing", async () => {
    const w = world([A]);
    await w.hookArm("docz create design x", created(A));
    let fire: () => void = () => undefined;
    const res = await w.stop(
      () => {
        fire();
        return Promise.resolve();
      },
      {
        onSignal: (h) => {
          fire = h;
          return () => undefined;
        },
      },
    );
    expect(res.out).toBe("");
    expect(w.session().read().entries[0]?.state).toBe("dismissed");
  });

  test("the deadline is a dismissal", async () => {
    const w = world([A]);
    await w.hookArm("docz create design x", created(A));
    const res = await w.run(["--pending", "--hook", "--timeout", "0"], {
      stdin: stream(JSON.stringify({ session_id: SESSION, cwd: w.root })),
    });
    expect(res.out).toBe("");
    expect(w.session().read().entries[0]?.state).toBe("dismissed");
  });

  test("a second server is refused by the lease and prints nothing", async () => {
    const w = world([A]);
    await w.hookArm("docz create design x", created(A));
    Lease.acquire(w.session(), { ttlMs: 60_000 });
    const res = await w.stop(() => Promise.resolve());
    expect(res.code).toBe(0);
    expect(res.out).toBe("");
    expect(res.err).toContain("another docz-review");
    expect(w.session().read().entries[0]?.state).toBe("pending");
  });

  test("a missing docz is one stderr line and exit 0 under --hook", async () => {
    const w = world([A]);
    const res = await w.run(["--pending", "--hook"], {
      env: { DOCZ_REVIEW_STATE_DIR: w.base, PATH: "" },
      stdin: stream(JSON.stringify({ session_id: SESSION, cwd: w.root })),
    });
    expect(res.code).toBe(0);
    expect(res.out).toBe("");
    expect(res.err).toContain("docz not found");
  });
});

describe("a review by hand", () => {
  test("--json prints the records, each valid against review.schema.json", async () => {
    const w = world([A]);
    const res = await w.run([A.inspect.id, "--json"], {
      openBrowser: fakeBrowser(async (s, post) => {
        await post(`/api/review/${A.inspect.id}/submit`, {
          ...keyOf(s, 0, "s_1"),
          ...changes,
        });
      }),
    });
    const records = JSON.parse(res.out) as unknown[];
    expect(records).toHaveLength(1);
    expect(parseReview(JSON.stringify(records[0])).outcome).toBe("changes");
    expect(res.err).toMatch(/resume with --resume r_[0-9a-f]+/);
  });

  test("--no-open prints the URL to stderr", async () => {
    const w = world([A]);
    const res = await w.run([A.inspect.id, "--no-open", "--timeout", "0"]);
    expect(res.err).toMatch(/open http:\/\/127\.0\.0\.1:\d+\/\?t=[0-9a-f]{64}/);
    expect(res.out).toContain("no changes requested");
  });
});

describe("the other commands", () => {
  test("arm by hand validates and prints, recording nothing", async () => {
    const w = world([A]);
    const res = await w.run(["arm", A.inspect.id, "NOPE-1"]);
    expect(res.code).toBe(1);
    expect(res.out).toBe(`${A.inspect.id} ${A.inspect.path}\n`);
    expect(res.err).toContain("NOPE-1");
    expect((await w.run(["pending"])).out).toContain(
      "no documents under review",
    );
  });

  test("pending lists the session's documents", async () => {
    const w = world([A]);
    await w.hookArm("docz create design x", created(A));
    expect((await w.run(["pending"])).out).toBe(
      `${SESSION}\t${A.inspect.id}\tpending\tround 0\t${A.inspect.path}\n`,
    );
  });

  test("--version names this build, the contract, and docz", async () => {
    const res = await world().run(["--version"]);
    expect(res.out).toContain("docz-review dev");
    expect(res.out).toContain("schema_version 1");
    expect(res.out).toContain("docz: v2.2.0");
  });

  test("a bad flag is a usage error", async () => {
    const res = await world().run(["--bogus"]);
    expect(res.code).toBe(2);
    expect(res.err).toContain("usage:");
  });
});
