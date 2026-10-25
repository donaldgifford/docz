/*
 * Arming (DESIGN-0024 section 2). `docz-review arm --from-hook` is the
 * PostToolUse hook on Bash and the only writer of session state, since it
 * is the only process that receives the session id. It reads the request
 * to EOF and recognises two commands by word presence alone, never running
 * or interpreting the shell text:
 *
 *   docz create          the path in the JSON envelope `--format json`
 *                        printed, else every `Created <TYPE>: <path>` line
 *   docz-review arm ...  the explicit form the plugin's skill uses
 *
 * Every candidate is validated through `docz inspect --repo-root <cwd>`
 * before it is recorded. One that does not resolve is dropped and named
 * in a systemMessage, which the person sees and the agent does not. Any
 * other command produces no output at all.
 */
import { realpathSync } from "node:fs";
import { basename } from "node:path";

import type { DoczInspect } from "@/cli/__generated__/inspect";

import type { Docz, DoczScope } from "./docz";
import { type Entry, Session } from "./state";

/** The fields of a PostToolUse request arming reads. */
export interface HookRequest {
  session_id?: unknown;
  cwd?: unknown;
  tool_name?: unknown;
  tool_input?: { command?: unknown };
  tool_response?: unknown;
}

export interface Dropped {
  candidate: string;
  reason: string;
}

export interface ArmResult {
  armed: Entry[];
  dropped: Dropped[];
  /** What the hook prints on stdout: a systemMessage object, or nothing. */
  stdout: string;
}

const SEPARATOR = /&&|\|\||[;&|\n]/;
const WORD = /[^\s;&|()<>`]+/g;
const CREATED = /^Created [A-Z][A-Z0-9_-]*: (.+)$/gm;

/** A command's words, quotes removed, program paths reduced to names. */
function words(segment: string): string[] {
  return (segment.match(WORD) ?? []).map((w) =>
    w.replace(/^['"]+|['"]+$/g, ""),
  );
}

/** Whether `name` appears as a word, `/usr/local/bin/docz` included. */
export function hasWord(command: string, name: string): boolean {
  return words(command).some((w) => w === name || basename(w) === name);
}

/** The arguments of every `docz-review arm …` in a command. */
export function reviewArmArgs(command: string): string[] {
  const out: string[] = [];
  for (const segment of command.split(SEPARATOR)) {
    const ws = words(segment);
    const at = ws.findIndex(
      (w, i) => basename(w) === "docz-review" && ws[i + 1] === "arm",
    );
    if (at !== -1) {
      out.push(...ws.slice(at + 2).filter((w) => !w.startsWith("-")));
    }
  }
  return out;
}

/** Every string anywhere in a tool response, in order. */
export function responseText(resp: unknown): string {
  if (typeof resp === "string") {
    return resp;
  }
  if (Array.isArray(resp)) {
    return resp.map(responseText).join("\n");
  }
  if (resp !== null && typeof resp === "object") {
    return Object.values(resp).map(responseText).join("\n");
  }
  return "";
}

/**
 * The paths `docz create` reported: each `--format json` envelope (a
 * multi-line object opening at column 0), else each `Created` line.
 */
export function createdPaths(text: string): string[] {
  const lines = text.split("\n");
  const fromJson: string[] = [];
  for (let i = 0; i < lines.length; i++) {
    if (!lines[i]?.startsWith("{")) {
      continue;
    }
    for (let j = i; j < lines.length; j++) {
      if (!lines[j]?.startsWith("}")) {
        continue;
      }
      const path = envelopePath(lines.slice(i, j + 1).join("\n"));
      if (path !== null) {
        fromJson.push(path);
        i = j;
        break;
      }
    }
  }
  if (fromJson.length > 0) {
    return fromJson;
  }
  return [...text.matchAll(CREATED)].map((m) => (m[1] ?? "").trim());
}

function envelopePath(block: string): string | null {
  try {
    const v = JSON.parse(block) as Record<string, unknown>;
    return typeof v.schema_version === "number" &&
      typeof v.id === "string" &&
      typeof v.path === "string"
      ? v.path
      : null;
  } catch {
    return null;
  }
}

/** The candidates a request names, in order, without duplicates. */
export function candidates(req: HookRequest): string[] {
  if (req.tool_name !== "Bash" || typeof req.tool_input?.command !== "string") {
    return [];
  }
  const command = req.tool_input.command;
  const found: string[] = [];
  if (hasWord(command, "docz")) {
    found.push(...createdPaths(responseText(req.tool_response)));
  }
  if (hasWord(command, "docz-review")) {
    found.push(...reviewArmArgs(command));
  }
  return [...new Set(found)];
}

export interface ArmDeps {
  docz: Docz;
  /** The state directory (stateRoot). */
  base: string;
}

/** Resolves each target through `docz inspect`, keeping the failures. */
export async function resolveTargets(
  docz: Docz,
  targets: readonly string[],
  scope: DoczScope,
): Promise<{ resolved: DoczInspect[]; dropped: Dropped[] }> {
  const resolved: DoczInspect[] = [];
  const dropped: Dropped[] = [];
  for (const candidate of targets) {
    try {
      resolved.push(await docz.inspect(candidate, scope));
    } catch (err) {
      dropped.push({ candidate, reason: (err as Error).message });
    }
  }
  return { resolved, dropped };
}

function systemMessage(dropped: readonly Dropped[]): string {
  if (dropped.length === 0) {
    return "";
  }
  const lines = dropped.map(
    (d) => `docz-review: not armed: ${d.candidate}: ${d.reason}`,
  );
  return `${JSON.stringify({ systemMessage: lines.join("\n") })}\n`;
}

/**
 * The hook: parse the request, validate every candidate, record the
 * ones that resolve as pending under the session. Never throws for a
 * request it cannot use; a hook that failed would only add noise.
 */
export async function armFromHook(
  raw: string,
  deps: ArmDeps,
): Promise<ArmResult> {
  let req: HookRequest;
  try {
    req = JSON.parse(raw) as HookRequest;
  } catch {
    return { armed: [], dropped: [], stdout: "" };
  }
  const found = candidates(req);
  if (found.length === 0) {
    return { armed: [], dropped: [], stdout: "" };
  }
  if (typeof req.session_id !== "string" || typeof req.cwd !== "string") {
    const dropped = found.map((candidate) => ({
      candidate,
      reason: "the hook request carried no session id or cwd",
    }));
    return { armed: [], dropped, stdout: systemMessage(dropped) };
  }

  let root: string;
  try {
    root = realpathSync(req.cwd);
  } catch (err) {
    const dropped = found.map((candidate) => ({
      candidate,
      reason: (err as Error).message,
    }));
    return { armed: [], dropped, stdout: systemMessage(dropped) };
  }

  const scope = { root, config: "" };
  const { resolved, dropped } = await resolveTargets(deps.docz, found, scope);
  const armed: Entry[] = [];
  let session: Session | undefined;
  for (const doc of resolved) {
    try {
      session ??= new Session({
        base: deps.base,
        root,
        session: req.session_id,
      });
      armed.push(
        session.arm({
          id: doc.id,
          type: doc.type,
          path: doc.path,
          root,
          config: scope.config,
          docs_dir: "",
        }),
      );
    } catch (err) {
      dropped.push({ candidate: doc.path, reason: (err as Error).message });
    }
  }
  return { armed, dropped, stdout: systemMessage(dropped) };
}
