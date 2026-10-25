/*
 * docz-review's session state (DESIGN-0024 Data Model). Nothing lives in
 * the repository: state is per person, per repository, per session, under
 *
 *   <state dir>/<sha256(root)>/<session id>/
 *     lock                     flock around every read-modify-write
 *     lease.json               one server owns a session at a time (lease.ts)
 *     state.json               one entry per document, replaced atomically
 *     reviews/<sha256(path)>/  round-N.snapshot.md, round-N.draft.json, round-N.json
 *
 * Directories are 0700 and files 0600, every read opens without following
 * a symlink, and every path component below the state dir is a hash or a
 * validated session id, never a name a document chose. Every operation is
 * synchronous: an flock is held by an open file description, so a lock
 * section that awaited could deadlock against another in the same process.
 */
import { createHash, randomBytes } from "node:crypto";
import {
  closeSync,
  constants,
  fsyncSync,
  lstatSync,
  mkdirSync,
  openSync,
  readdirSync,
  readFileSync,
  renameSync,
  rmSync,
  statSync,
  unlinkSync,
  writeSync,
} from "node:fs";
import { join } from "node:path";

import { dlopen, FFIType } from "bun:ffi";

/** A document's place in the review state machine. */
export type DocState =
  "pending" | "open" | "changes" | "approved" | "dismissed";

/** What moves a document between states. */
export type DocEvent = "arm" | "open" | "changes" | "approved" | "dismissed";

/** One document under review in a session. */
export interface Entry {
  /** sha256 of `path`: the document's directory under reviews/. */
  key: string;
  id: string;
  type: string;
  /** Repository-relative path. */
  path: string;
  /** Canonical repository root, symlinks resolved. */
  root: string;
  /** The --config file in effect, or empty for the root's .docz.yaml. */
  config: string;
  docs_dir: string;
  state: DocState;
  /** The last round opened; 0 before the first. */
  round: number;
  /**
   * `round` when the document was last armed. The round limit counts
   * from here, so re-arming lifts it while the counter, and with it the
   * diff against the previous round, carries on.
   */
  armed_round: number;
  /** content_sha256 at the last round's open. */
  digest: string;
  /** The submission recorded for the current round, if any. */
  submission_id: string;
  armed_at: string;
  updated_at: string;
}

export interface StateFile {
  version: 1;
  entries: Entry[];
}

/** A document to arm: what `docz inspect` and the hook resolved. */
export type Candidate = Pick<
  Entry,
  "id" | "type" | "path" | "root" | "config" | "docs_dir"
>;

/** Only a candidate's own fields, whatever object carried them. */
function candidateOf(c: Candidate): Candidate {
  const { id, type, path, root, config, docs_dir } = c;
  return { id, type, path, root, config, docs_dir };
}

/** Entries and session directories are kept this long after a write. */
export const EXPIRY_MS = 30 * 24 * 60 * 60 * 1000;

const LOCK_EX = 2;
const LOCK_NB = 4;
const LOCK_UN = 8;
const LOCK_WAIT_MS = 10_000;
const DIR_MODE = 0o700;
const FILE_MODE = 0o600;
const SESSION_ID = /^[A-Za-z0-9_-][A-Za-z0-9._-]{0,127}$/;

export class StateError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "StateError";
  }
}

export function sha256(s: string): string {
  return createHash("sha256").update(s).digest("hex");
}

/**
 * The state directory: DOCZ_REVIEW_STATE_DIR, else
 * $XDG_STATE_HOME/docz/review, else $HOME/.local/state/docz/review.
 */
export function stateRoot(env: Record<string, string | undefined>): string {
  if (env.DOCZ_REVIEW_STATE_DIR) {
    return env.DOCZ_REVIEW_STATE_DIR;
  }
  if (env.XDG_STATE_HOME) {
    return join(env.XDG_STATE_HOME, "docz", "review");
  }
  if (env.HOME) {
    return join(env.HOME, ".local", "state", "docz", "review");
  }
  throw new StateError("no state directory: set DOCZ_REVIEW_STATE_DIR or HOME");
}

/** A random id for a review started by hand, which has no session id. */
export function newReviewId(): string {
  return `r_${randomBytes(12).toString("hex")}`;
}

/**
 * The transition table. Re-arming keeps the round counter, so the next
 * round still has a previous one to diff against; an `open` entry is
 * left to the server that holds it.
 */
export function nextState(from: DocState, event: DocEvent): DocState {
  switch (event) {
    case "arm":
      return from === "open" ? "open" : "pending";
    case "open":
      // `open` again is a server taking over an expired lease.
      if (from === "pending" || from === "changes" || from === "open") {
        return "open";
      }
      break;
    case "changes":
    case "approved":
      if (from === "open") {
        return event;
      }
      break;
    case "dismissed":
      // pending and changes are dismissed by the round limit.
      if (from === "open" || from === "pending" || from === "changes") {
        return "dismissed";
      }
      break;
  }
  throw new StateError(`no transition from ${from} on ${event}`);
}

// --- files -----------------------------------------------------------

/** Reads a file without following a symlink; null when absent. */
export function readNoFollow(path: string): Buffer | null {
  let fd: number;
  try {
    fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return null;
    }
    throw err;
  }
  try {
    return readFileSync(fd);
  } finally {
    closeSync(fd);
  }
}

/**
 * Replaces a file atomically: a fresh 0600 temp file, fsynced, renamed
 * over the target, and the directory fsynced so the rename survives a
 * crash. A reader sees the old bytes or the new, never half of either.
 */
export function writeAtomic(path: string, data: string | Uint8Array): void {
  const tmp = `${path}.${randomBytes(6).toString("hex")}.tmp`;
  const fd = openSync(
    tmp,
    constants.O_WRONLY |
      constants.O_CREAT |
      constants.O_EXCL |
      constants.O_NOFOLLOW,
    FILE_MODE,
  );
  try {
    const bytes = typeof data === "string" ? Buffer.from(data) : data;
    let off = 0;
    while (off < bytes.length) {
      off += writeSync(fd, bytes, off);
    }
    fsyncSync(fd);
  } catch (err) {
    closeSync(fd);
    unlinkSync(tmp);
    throw err;
  }
  closeSync(fd);
  renameSync(tmp, path);
  const dir = openSync(join(path, ".."), constants.O_RDONLY);
  try {
    fsyncSync(dir);
  } finally {
    closeSync(dir);
  }
}

/** Creates a directory chain at 0700 and refuses a symlink in it. */
function ensureDir(base: string, ...parts: string[]): string {
  mkdirSync(base, { recursive: true, mode: DIR_MODE });
  let dir = base;
  for (const part of parts) {
    dir = join(dir, part);
    try {
      mkdirSync(dir, { mode: DIR_MODE });
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== "EEXIST") {
        throw err;
      }
    }
    const st = lstatSync(dir);
    if (!st.isDirectory() || st.isSymbolicLink()) {
      throw new StateError(`${dir} is not a directory`);
    }
  }
  return dir;
}

// --- the lock --------------------------------------------------------

let libc: { flock: (fd: number, op: number) => number } | undefined;

function flock(fd: number, op: number): number {
  libc ??= dlopen(process.platform === "darwin" ? "libc.dylib" : "libc.so.6", {
    flock: { args: [FFIType.i32, FFIType.i32], returns: FFIType.i32 },
  }).symbols;
  return libc.flock(fd, op);
}

/**
 * Runs fn holding an exclusive flock on `path`. The wait is bounded, so
 * a wedged holder is an error rather than a hang.
 */
export function withLock<T>(path: string, fn: () => T): T {
  const fd = openSync(
    path,
    constants.O_RDWR | constants.O_CREAT | constants.O_NOFOLLOW,
    FILE_MODE,
  );
  try {
    const deadline = Date.now() + LOCK_WAIT_MS;
    while (flock(fd, LOCK_EX | LOCK_NB) !== 0) {
      if (Date.now() > deadline) {
        throw new StateError(`timed out waiting for ${path}`);
      }
      Bun.sleepSync(5);
    }
    try {
      return fn();
    } finally {
      flock(fd, LOCK_UN);
    }
  } finally {
    closeSync(fd);
  }
}

// --- a session -------------------------------------------------------

export interface SessionOptions {
  /** The state directory (stateRoot). */
  base: string;
  /** The canonical repository root. */
  root: string;
  /** The hook's session_id, or a review id for a review by hand. */
  session: string;
  now?: () => Date;
}

/** One session's state directory. */
export class Session {
  readonly dir: string;
  private readonly now: () => Date;

  constructor(opts: SessionOptions) {
    if (!SESSION_ID.test(opts.session)) {
      throw new StateError(
        `session id ${JSON.stringify(opts.session)} is not valid`,
      );
    }
    this.now = opts.now ?? (() => new Date());
    this.dir = ensureDir(opts.base, sha256(opts.root), opts.session);
  }

  get lockPath(): string {
    return join(this.dir, "lock");
  }

  /** Runs fn under the session's lock. */
  locked<T>(fn: () => T): T {
    return withLock(this.lockPath, fn);
  }

  /** Reads state.json; call under the lock when the result is written back. */
  read(): StateFile {
    const raw = readNoFollow(join(this.dir, "state.json"));
    if (raw === null) {
      return { version: 1, entries: [] };
    }
    const parsed = JSON.parse(raw.toString("utf8")) as Partial<StateFile>;
    if (parsed.version !== 1 || !Array.isArray(parsed.entries)) {
      throw new StateError(
        `${this.dir}/state.json is not a version 1 state file`,
      );
    }
    // armed_round arrived after version 1's first entries; 0 is what
    // an entry written without it means.
    const entries = parsed.entries.map((e) => ({
      ...e,
      armed_round: (e as Partial<Entry>).armed_round ?? 0,
    }));
    return { version: 1, entries };
  }

  /**
   * A read-modify-write of state.json under the lock. Entries not written
   * for EXPIRY_MS are dropped on the way, except an open one. Returns
   * what fn returns.
   */
  update<T>(fn: (s: StateFile) => T): T {
    return this.locked(() => {
      const s = this.read();
      const out = fn(s);
      const cutoff = this.now().getTime() - EXPIRY_MS;
      s.entries = s.entries.filter(
        (e) => e.state === "open" || Date.parse(e.updated_at) >= cutoff,
      );
      writeAtomic(
        join(this.dir, "state.json"),
        `${JSON.stringify(s, null, 2)}\n`,
      );
      return out;
    });
  }

  /** Arms a document: a new entry, or back to pending with its round kept. */
  arm(c: Candidate): Entry {
    return this.update((s) => {
      const at = this.stamp();
      const key = sha256(c.path);
      const found = s.entries.find((e) => e.key === key);
      if (found === undefined) {
        const armed: Entry = {
          ...candidateOf(c),
          key,
          state: "pending",
          round: 0,
          armed_round: 0,
          digest: "",
          submission_id: "",
          armed_at: at,
          updated_at: at,
        };
        s.entries.push(armed);
        return armed;
      }
      return Object.assign(found, candidateOf(c), {
        state: nextState(found.state, "arm"),
        armed_round: found.round,
        armed_at: at,
        updated_at: at,
      });
    });
  }

  /**
   * Applies an event to one entry under the lock, with any fields the
   * event sets. Opening from pending or changes starts the next round.
   */
  apply(key: string, event: DocEvent, fields: Partial<Entry> = {}): Entry {
    return this.update((s) => {
      const e = s.entries.find((x) => x.key === key);
      if (e === undefined) {
        throw new StateError(`no document ${key} in this session`);
      }
      const from = e.state;
      e.state = nextState(from, event);
      if (event === "open" && from !== "open") {
        e.round += 1;
        e.submission_id = "";
      }
      return Object.assign(e, fields, { updated_at: this.stamp() });
    });
  }

  /**
   * Settles entries a crashed server left open: a round whose record is
   * on disk takes the record's outcome and submission id; one without a
   * record stays open for the server now holding the lease.
   */
  reconcile(): Entry[] {
    return this.update((s) => {
      for (const e of s.entries) {
        if (e.state !== "open") {
          continue;
        }
        const raw = readNoFollow(this.recordPath(e.key, e.round));
        if (raw === null) {
          continue;
        }
        const rec = JSON.parse(raw.toString("utf8")) as {
          outcome: "changes" | "approved" | "dismissed";
          submission_id: string;
        };
        e.state = nextState("open", rec.outcome);
        e.submission_id = rec.submission_id;
        e.updated_at = this.stamp();
      }
      return s.entries;
    });
  }

  // --- per-document files ---------------------------------------------

  reviewDir(key: string): string {
    return ensureDir(this.dir, "reviews", key);
  }

  recordPath(key: string, round: number): string {
    return join(this.dir, "reviews", key, `round-${String(round)}.json`);
  }

  writeRecord(key: string, round: number, record: unknown): void {
    this.reviewDir(key);
    writeAtomic(
      this.recordPath(key, round),
      `${JSON.stringify(record, null, 2)}\n`,
    );
  }

  readRecord(key: string, round: number): unknown {
    const raw = readNoFollow(this.recordPath(key, round));
    return raw === null ? null : JSON.parse(raw.toString("utf8"));
  }

  writeSnapshot(key: string, round: number, content: string): void {
    writeAtomic(
      join(this.reviewDir(key), `round-${String(round)}.snapshot.md`),
      content,
    );
  }

  readSnapshot(key: string, round: number): string | null {
    const raw = readNoFollow(
      join(this.dir, "reviews", key, `round-${String(round)}.snapshot.md`),
    );
    return raw === null ? null : raw.toString("utf8");
  }

  writeDraft(key: string, round: number, draft: unknown): void {
    writeAtomic(
      join(this.reviewDir(key), `round-${String(round)}.draft.json`),
      JSON.stringify(draft),
    );
  }

  readDraft(key: string, round: number): unknown {
    const raw = readNoFollow(
      join(this.dir, "reviews", key, `round-${String(round)}.draft.json`),
    );
    return raw === null ? null : JSON.parse(raw.toString("utf8"));
  }

  deleteDraft(key: string, round: number): void {
    rmSync(
      join(this.dir, "reviews", key, `round-${String(round)}.draft.json`),
      {
        force: true,
      },
    );
  }

  private stamp(): string {
    return this.now().toISOString();
  }
}

/** The newest modification time anywhere under a directory, in ms. */
function newest(dir: string): number {
  let t = statSync(dir).mtimeMs;
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    const st = lstatSync(p);
    t = Math.max(t, st.isDirectory() ? newest(p) : st.mtimeMs);
  }
  return t;
}

/**
 * Removes every session directory not written for EXPIRY_MS, whatever
 * its entries hold, and every repository directory left empty. Returns
 * the session directories removed.
 */
export function purge(base: string, now: Date = new Date()): string[] {
  const removed: string[] = [];
  let roots: string[];
  try {
    roots = readdirSync(base);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return removed;
    }
    throw err;
  }
  const cutoff = now.getTime() - EXPIRY_MS;
  for (const r of roots) {
    const rootDir = join(base, r);
    if (!lstatSync(rootDir).isDirectory()) {
      continue;
    }
    for (const s of readdirSync(rootDir)) {
      const dir = join(rootDir, s);
      if (lstatSync(dir).isDirectory() && newest(dir) < cutoff) {
        rmSync(dir, { recursive: true, force: true });
        removed.push(dir);
      }
    }
    if (readdirSync(rootDir).length === 0) {
      rmSync(rootDir, { recursive: true, force: true });
    }
  }
  return removed;
}
