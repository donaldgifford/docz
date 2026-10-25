import { describe, expect, test } from "bun:test";
import {
  existsSync,
  mkdtempSync,
  readdirSync,
  statSync,
  symlinkSync,
  utimesSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import {
  type Candidate,
  EXPIRY_MS,
  nextState,
  purge,
  Session,
  sha256,
  stateRoot,
  StateError,
} from "./state";

const base = () => mkdtempSync(join(tmpdir(), "docz-review-state-"));

const doc = (path = "docs/design/0001-a.md"): Candidate => ({
  id: "DESIGN-0001",
  type: "design",
  path,
  root: "/repo",
  config: "",
  docs_dir: "docs",
});

describe("stateRoot", () => {
  test("prefers DOCZ_REVIEW_STATE_DIR, then XDG_STATE_HOME, then HOME", () => {
    expect(
      stateRoot({ DOCZ_REVIEW_STATE_DIR: "/s", XDG_STATE_HOME: "/x" }),
    ).toBe("/s");
    expect(stateRoot({ XDG_STATE_HOME: "/x", HOME: "/h" })).toBe(
      "/x/docz/review",
    );
    expect(stateRoot({ HOME: "/h" })).toBe("/h/.local/state/docz/review");
    expect(() => stateRoot({})).toThrow(StateError);
  });
});

describe("nextState", () => {
  test("follows the Data Model's table", () => {
    expect(nextState("approved", "arm")).toBe("pending");
    expect(nextState("dismissed", "arm")).toBe("pending");
    expect(nextState("open", "arm")).toBe("open");
    expect(nextState("pending", "open")).toBe("open");
    expect(nextState("changes", "open")).toBe("open");
    expect(nextState("open", "changes")).toBe("changes");
    expect(nextState("open", "approved")).toBe("approved");
    expect(nextState("pending", "dismissed")).toBe("dismissed");
    expect(() => nextState("approved", "open")).toThrow(StateError);
    expect(() => nextState("pending", "approved")).toThrow(StateError);
    expect(() => nextState("dismissed", "changes")).toThrow(StateError);
  });
});

describe("Session", () => {
  test("lays out sha256(root)/session at 0700 with 0600 files", () => {
    const b = base();
    const s = new Session({ base: b, root: "/repo", session: "abc-123" });
    expect(s.dir).toBe(join(b, sha256("/repo"), "abc-123"));
    s.arm(doc());
    expect(statSync(s.dir).mode & 0o777).toBe(0o700);
    expect(statSync(join(s.dir, "state.json")).mode & 0o777).toBe(0o600);
    expect(statSync(join(s.dir, "lock")).mode & 0o777).toBe(0o600);
  });

  test("refuses a session id that could escape its directory", () => {
    for (const bad of ["..", "../x", "a/b", "", ".hidden"]) {
      expect(
        () => new Session({ base: base(), root: "/r", session: bad }),
      ).toThrow(StateError);
    }
  });

  test("refuses to read state.json through a symlink", () => {
    const b = base();
    const s = new Session({ base: b, root: "/repo", session: "s" });
    const target = join(b, "elsewhere.json");
    writeFileSync(target, '{"version":1,"entries":[]}');
    symlinkSync(target, join(s.dir, "state.json"));
    expect(() => s.read()).toThrow();
  });

  test("keeps the round counter across re-arming", () => {
    const s = new Session({ base: base(), root: "/repo", session: "s" });
    const e = s.arm(doc());
    expect(e.state).toBe("pending");
    expect(s.apply(e.key, "open", { digest: "d1" }).round).toBe(1);
    s.apply(e.key, "changes");
    expect(s.apply(e.key, "open").round).toBe(2);
    s.apply(e.key, "approved");
    const again = s.arm(doc());
    expect(again).toMatchObject({ state: "pending", round: 2 });
    expect(s.apply(e.key, "open").round).toBe(3);
  });

  test("an illegal transition leaves state.json as it was", () => {
    const s = new Session({ base: base(), root: "/repo", session: "s" });
    const e = s.arm(doc());
    expect(() => s.apply(e.key, "approved")).toThrow(StateError);
    expect(s.read().entries[0]?.state).toBe("pending");
  });

  test("expires an entry not written for 30 days, never an open one", () => {
    let now = new Date("2026-01-01T00:00:00Z");
    const s = new Session({
      base: base(),
      root: "/r",
      session: "s",
      now: () => now,
    });
    s.arm(doc("docs/a.md"));
    const open = s.arm(doc("docs/b.md"));
    s.apply(open.key, "open");
    now = new Date(now.getTime() + EXPIRY_MS + 1000);
    s.arm(doc("docs/c.md"));
    expect(s.read().entries.map((e) => e.path)).toEqual([
      "docs/b.md",
      "docs/c.md",
    ]);
  });

  test("two processes writing at once lose nothing", async () => {
    const b = base();
    const writer = join(import.meta.dir, "testdata", "writer.ts");
    const procs = ["x", "y"].map((p) =>
      Bun.spawn(["bun", writer, b, p, "40"], { stderr: "inherit" }),
    );
    expect(await Promise.all(procs.map((p) => p.exited))).toEqual([0, 0]);
    const s = new Session({ base: b, root: "/repo", session: "s1" });
    expect(s.read().entries).toHaveLength(80);
    expect(readdirSync(s.dir).filter((n) => n.endsWith(".tmp"))).toEqual([]);
  });

  test("a crash between the record and the state is settled by the record", () => {
    const s = new Session({ base: base(), root: "/repo", session: "s" });
    const a = s.arm(doc("docs/a.md"));
    const b = s.arm(doc("docs/b.md"));
    s.apply(a.key, "open");
    s.apply(b.key, "open");
    // The server wrote a's record and died before updating the state.
    s.writeRecord(a.key, 1, { outcome: "changes", submission_id: "s_1" });
    const entries = s.reconcile();
    expect(entries.find((e) => e.key === a.key)).toMatchObject({
      state: "changes",
      submission_id: "s_1",
    });
    expect(entries.find((e) => e.key === b.key)?.state).toBe("open");
  });

  test("snapshots, drafts, and records live under reviews/<sha256(path)>", () => {
    const s = new Session({ base: base(), root: "/repo", session: "s" });
    const e = s.arm(doc());
    s.writeSnapshot(e.key, 1, "# one\n");
    s.writeDraft(e.key, 1, { questions: {} });
    expect(
      existsSync(join(s.dir, "reviews", sha256(e.path), "round-1.snapshot.md")),
    ).toBe(true);
    expect(s.readSnapshot(e.key, 1)).toBe("# one\n");
    expect(s.readDraft(e.key, 1)).toEqual({ questions: {} });
    s.deleteDraft(e.key, 1);
    expect(s.readDraft(e.key, 1)).toBeNull();
    expect(s.readSnapshot(e.key, 2)).toBeNull();
  });
});

describe("purge", () => {
  test("removes a session not written for 30 days and its empty parent", () => {
    const b = base();
    const old = new Session({ base: b, root: "/old", session: "s" });
    old.arm(doc());
    const fresh = new Session({ base: b, root: "/new", session: "s" });
    fresh.arm(doc());
    const past = (Date.now() - EXPIRY_MS - 60_000) / 1000;
    for (const name of ["", "state.json", "lock"]) {
      utimesSync(join(old.dir, name), past, past);
    }
    expect(purge(b)).toEqual([old.dir]);
    expect(existsSync(join(b, sha256("/old")))).toBe(false);
    expect(existsSync(fresh.dir)).toBe(true);
  });

  test("is a no-op on a missing state directory", () => {
    expect(purge(join(base(), "absent"))).toEqual([]);
  });
});
