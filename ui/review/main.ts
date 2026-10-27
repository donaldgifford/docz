/*
 * The docz-review command (DESIGN-0024, "API / Interface Changes"):
 *
 *   docz-review <id|path>… [--hook|--json] [--repo-root] [--config]
 *               [--no-open] [--timeout] [--grace] [--max-rounds] [--port]
 *   docz-review --resume <review id>
 *   docz-review --pending --hook          the Stop hook
 *   docz-review arm <id|path>… | --from-hook
 *   docz-review pending [--purge] [--session <id>]
 *   docz-review --version
 *
 * Under --hook stdout carries at most one JSON object, printed once and
 * last: the block decision when a document ended in changes, nothing
 * otherwise. Every diagnostic goes to stderr. The URL is a credential, so
 * it goes to stderr too, and only when the browser was not opened.
 */
import { spawn } from "node:child_process";
import { readdirSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { parseArgs } from "node:util";

import { SUPPORTED } from "@/cli/validate";

import { armFromHook, resolveTargets } from "./arm";
import { Docz, resolveDocz } from "./docz";
import { Lease, LeaseHeldError } from "./lease";
import { Review } from "./round";
import { type AssetSource, diskAssets, readAll, startServer } from "./server";
import {
  type Entry,
  newReviewId,
  purge,
  Session,
  sha256,
  stateRoot,
} from "./state";

declare const DOCZ_REVIEW_VERSION: string | undefined;

/** This build's version, set by `bun build --define`; "dev" otherwise. */
export const VERSION =
  typeof DOCZ_REVIEW_VERSION === "string" ? DOCZ_REVIEW_VERSION : "dev";

/** The plugin's Stop hook timeout, in seconds (design question 1). */
export const HOOK_TIMEOUT_S = 86_400;
const DEFAULTS = {
  timeoutS: HOOK_TIMEOUT_S - 60,
  graceS: 120,
  maxRounds: 10,
};
const SCHEMAS = ["inspect", "validate", "create", "review"];

/** Everything main touches outside itself, so a test can stand in. */
export interface Io {
  argv: string[];
  env: Record<string, string | undefined>;
  cwd: string;
  stdin: ReadableStream<Uint8Array>;
  out: (s: string) => void;
  err: (s: string) => void;
  /** Opens the URL; false when no browser could be started. */
  openBrowser: (url: string) => Promise<boolean>;
  assets: AssetSource;
  /** Calls handler on SIGTERM, SIGHUP, or SIGINT; returns an unsubscribe. */
  onSignal: (handler: () => void) => () => void;
  now?: () => Date;
}

const USAGE = `usage:
  docz-review <id|path>... [--json] [--repo-root DIR] [--config FILE] [--no-open]
                           [--timeout S] [--grace S] [--max-rounds N] [--port N]
  docz-review --resume <review id> [--repo-root DIR]
  docz-review --pending --hook
  docz-review arm <id|path>... | arm --from-hook
  docz-review pending [--purge] [--session ID] [--repo-root DIR]
  docz-review --version
`;

const OPTIONS = {
  hook: { type: "boolean" },
  json: { type: "boolean" },
  pending: { type: "boolean" },
  resume: { type: "string" },
  "repo-root": { type: "string" },
  config: { type: "string" },
  "no-open": { type: "boolean" },
  timeout: { type: "string" },
  grace: { type: "string" },
  "max-rounds": { type: "string" },
  port: { type: "string" },
  "from-hook": { type: "boolean" },
  purge: { type: "boolean" },
  session: { type: "string" },
  version: { type: "boolean" },
  help: { type: "boolean", short: "h" },
} as const;

type Flags = ReturnType<typeof parse>["values"];

function parse(argv: string[]) {
  return parseArgs({
    args: argv,
    options: OPTIONS,
    allowPositionals: true,
    strict: true,
  });
}

class UsageError extends Error {}

function count(
  flags: Flags,
  name: "timeout" | "grace" | "max-rounds" | "port",
): number | undefined {
  const raw = flags[name];
  if (raw === undefined) {
    return undefined;
  }
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 0) {
    throw new UsageError(`--${name} must be a whole number, not ${raw}`);
  }
  return n;
}

export async function main(io: Io): Promise<number> {
  let flags: Flags;
  let positionals: string[];
  try {
    ({ values: flags, positionals } = parse(io.argv));
  } catch (err) {
    io.err(`docz-review: ${(err as Error).message}\n${USAGE}`);
    return 2;
  }
  try {
    if (flags.help === true) {
      io.out(USAGE);
      return 0;
    }
    if (flags.version === true) {
      return await version(io);
    }
    if (positionals[0] === "arm") {
      return await arm(io, flags, positionals.slice(1));
    }
    if (positionals[0] === "pending") {
      return pending(io, flags);
    }
    return await review(io, flags, positionals);
  } catch (err) {
    if (err instanceof UsageError) {
      io.err(`docz-review: ${err.message}\n${USAGE}`);
      return 2;
    }
    io.err(`docz-review: ${(err as Error).message}\n`);
    // A hook that fails only adds noise to the session; say why and exit 0.
    return flags.hook === true ? 0 : 1;
  }
}

function repoRoot(io: Io, flags: Flags): string {
  return realpathSync(flags["repo-root"] ?? io.cwd);
}

async function version(io: Io): Promise<number> {
  const bin = resolveDocz(io.env);
  let docz = "not found";
  if (bin !== null) {
    try {
      docz = `${await new Docz(bin).version()} (${bin})`;
    } catch (err) {
      docz = `${bin}: ${(err as Error).message}`;
    }
  }
  io.out(
    `docz-review ${VERSION}\n` +
      `contract: schema_version ${String(SUPPORTED.schemaVersion)} (${SCHEMAS.join(", ")}), docz ${SUPPORTED.minDocz} or later\n` +
      `docz: ${docz}\n`,
  );
  return 0;
}

async function arm(io: Io, flags: Flags, targets: string[]): Promise<number> {
  const bin = resolveDocz(io.env);
  if (flags["from-hook"] === true) {
    const raw = await readAll(io.stdin);
    if (bin === null) {
      return 0;
    }
    const res = await armFromHook(raw, {
      docz: new Docz(bin),
      base: stateRoot(io.env),
    });
    io.out(res.stdout);
    return 0;
  }
  if (targets.length === 0) {
    throw new UsageError("arm needs at least one id or path, or --from-hook");
  }
  if (bin === null) {
    io.err(
      "docz-review: docz not found; set DOCZ_REVIEW_DOCZ or put docz on PATH\n",
    );
    return 2;
  }
  const scope = { root: repoRoot(io, flags), config: flags.config ?? "" };
  const { resolved, dropped } = await resolveTargets(
    new Docz(bin),
    targets,
    scope,
  );
  for (const d of resolved) {
    io.out(`${d.id} ${d.path}\n`);
  }
  for (const d of dropped) {
    io.err(`docz-review: ${d.candidate}: ${d.reason}\n`);
  }
  return dropped.length > 0 ? 1 : 0;
}

function pending(io: Io, flags: Flags): number {
  const base = stateRoot(io.env);
  if (flags.purge === true) {
    for (const dir of purge(base, io.now?.())) {
      io.out(`purged ${dir}\n`);
    }
    return 0;
  }
  const root = repoRoot(io, flags);
  let sessions: string[];
  try {
    sessions = readdirSync(join(base, sha256(root)));
  } catch {
    sessions = [];
  }
  let any = false;
  for (const id of sessions) {
    if (flags.session !== undefined && id !== flags.session) {
      continue;
    }
    for (const e of new Session({ base, root, session: id }).read().entries) {
      any = true;
      io.out(
        `${id}\t${e.id}\t${e.state}\tround ${String(e.round)}\t${e.path}\n`,
      );
    }
  }
  if (!any) {
    io.out(`no documents under review in ${root}\n`);
  }
  return 0;
}

/** Entries a server opens: pending and changes, and open ones it now leases. */
const openable = (e: Entry) =>
  e.state === "pending" || e.state === "changes" || e.state === "open";

async function review(
  io: Io,
  flags: Flags,
  targets: string[],
): Promise<number> {
  const hook = flags.hook === true;
  const now = io.now ?? (() => new Date());
  const timeoutS = count(flags, "timeout") ?? DEFAULTS.timeoutS;
  const graceS = count(flags, "grace") ?? DEFAULTS.graceS;
  const maxRounds = count(flags, "max-rounds") ?? DEFAULTS.maxRounds;
  const port = count(flags, "port") ?? Number(io.env.DOCZ_REVIEW_PORT ?? "0");
  if (
    flags.pending !== true &&
    flags.resume === undefined &&
    targets.length === 0
  ) {
    throw new UsageError("name a document, or use --pending or --resume");
  }

  // The hook's stdin is read to EOF before anything else, then ignored.
  let request: { session_id?: unknown; cwd?: unknown } = {};
  if (hook) {
    try {
      request = JSON.parse(await readAll(io.stdin)) as typeof request;
    } catch {
      request = {};
    }
  }

  const bin = resolveDocz(io.env);
  if (bin === null) {
    io.err(
      "docz-review: docz not found; install docz or set DOCZ_REVIEW_DOCZ\n",
    );
    return hook ? 0 : 2;
  }
  const docz = new Docz(bin);
  await docz.version();

  const base = stateRoot(io.env);
  let root: string;
  let sessionId: string;
  if (flags.pending === true) {
    if (
      typeof request.session_id !== "string" ||
      typeof request.cwd !== "string"
    ) {
      if (!hook) {
        throw new UsageError(
          "--pending reads the Stop hook's request; use it with --hook",
        );
      }
      return 0;
    }
    root = realpathSync(request.cwd);
    sessionId = request.session_id;
  } else if (flags.resume !== undefined) {
    root = repoRoot(io, flags);
    sessionId = flags.resume;
  } else {
    root = repoRoot(io, flags);
    sessionId = newReviewId();
  }
  const session = new Session({ base, root, session: sessionId, now });

  if (targets.length > 0 && flags.pending !== true) {
    const scope = { root, config: flags.config ?? "" };
    const { resolved, dropped } = await resolveTargets(docz, targets, scope);
    for (const d of dropped) {
      io.err(`docz-review: ${d.candidate}: ${d.reason}\n`);
    }
    for (const d of resolved) {
      session.arm({
        id: d.id,
        type: d.type,
        path: d.path,
        root,
        config: scope.config,
        docs_dir: "",
      });
    }
    if (resolved.length === 0) {
      return 2;
    }
    io.err(
      `docz-review: review ${sessionId} (resume with --resume ${sessionId})\n`,
    );
  }

  if (!session.read().entries.some(openable)) {
    return 0;
  }
  let lease: Lease;
  try {
    lease = Lease.acquire(session, { ttlMs: 90_000, now });
  } catch (err) {
    if (err instanceof LeaseHeldError) {
      io.err(`docz-review: ${err.message}\n`);
      return hook ? 0 : 1;
    }
    throw err;
  }

  const entries = session.reconcile().filter(openable);
  const deadline = new Date(now().getTime() + timeoutS * 1000);
  const opened = await Review.open(entries, {
    session,
    docz,
    reviewId: sessionId,
    deadline,
    maxRounds,
    now,
  });
  for (const e of opened.limited) {
    io.err(
      `docz-review: ${e.id} reached ${String(maxRounds)} rounds; dismissed (re-arm it to continue)\n`,
    );
  }
  for (const f of opened.failed) {
    io.err(`docz-review: ${f.entry.id}: ${f.reason}\n`);
  }
  if (opened.review.size === 0) {
    lease.release();
    return hook || opened.failed.length === 0 ? 0 : 1;
  }

  const srv = startServer({
    review: opened.review,
    assets: io.assets,
    port,
    lease,
    graceMs: graceS * 1000,
    deadline,
  });
  const off = io.onSignal(() => {
    srv.end("signal");
  });
  const shown = flags["no-open"] !== true && (await io.openBrowser(srv.url));
  if (!shown) {
    io.err(`docz-review: open ${srv.url}\n`);
  }
  const reason = await srv.done;
  off();

  const feedback = opened.review.feedback();
  if (hook) {
    if (feedback !== null) {
      io.out(`${JSON.stringify({ decision: "block", reason: feedback })}\n`);
    }
    return 0;
  }
  if (flags.json === true) {
    io.out(`${JSON.stringify(opened.review.records(), null, 2)}\n`);
    return 0;
  }
  if (feedback !== null) {
    io.out(`${feedback}\n`);
  } else {
    io.out(`docz review: ended (${reason}) with no changes requested\n`);
  }
  return 0;
}

/** Opens a URL with DOCZ_REVIEW_BROWSER, BROWSER, or the platform's opener. */
export function browserOpener(
  env: Record<string, string | undefined>,
): (url: string) => Promise<boolean> {
  const cmd =
    env.DOCZ_REVIEW_BROWSER ??
    env.BROWSER ??
    (process.platform === "darwin" ? "open" : "xdg-open");
  return (url) =>
    new Promise((resolve) => {
      const child = spawn(cmd, [url], { stdio: "ignore", detached: true });
      child.once("error", () => {
        resolve(false);
      });
      child.once("spawn", () => {
        child.unref();
        resolve(true);
      });
    });
}

/** The real process's Io, with the page's assets from `assets`. */
export function processIo(assets: AssetSource): Io {
  return {
    argv: process.argv.slice(2),
    env: process.env,
    cwd: process.cwd(),
    stdin: Bun.stdin.stream(),
    out: (s) => process.stdout.write(s),
    err: (s) => process.stderr.write(s),
    openBrowser: browserOpener(process.env),
    assets,
    onSignal: (handler) => {
      const signals = ["SIGTERM", "SIGHUP", "SIGINT"] as const;
      for (const s of signals) {
        process.on(s, handler);
      }
      return () => {
        for (const s of signals) {
          process.off(s, handler);
        }
      };
    },
  };
}

if (import.meta.main) {
  const dir =
    process.env.DOCZ_REVIEW_ASSETS ??
    join(import.meta.dir, "..", "dist-review");
  process.exitCode = await main(processIo(diskAssets(dir)));
}
