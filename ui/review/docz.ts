/*
 * The docz CLI as docz-review calls it (DESIGN-0024 sections 3, 4, 6).
 * Every call is execFile with an argument list, never a shell, run from
 * the entry's repository root with its recorded --repo-root and --config,
 * and every output is parsed through src/cli/validate.ts, so a docz that
 * drops or retypes a field is refused rather than half-read.
 *
 * A failure to run docz, or a non-zero exit with no report, is a
 * DoczError: it is never turned into an empty findings report, because
 * "docz found nothing" and "docz did not run" must not look alike.
 */
import { execFile } from "node:child_process";

import {
  checkSchemaVersion,
  MAX_OUTPUT_BYTES,
  parseInspect,
  parseValidate,
  SUPPORTED,
} from "@/cli/validate";
import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { DoczValidate, Finding } from "@/cli/__generated__/validate";

export type DoczErrorKind = "spawn" | "exit" | "digest";

export class DoczError extends Error {
  constructor(
    readonly kind: DoczErrorKind,
    message: string,
    readonly exitCode?: number,
  ) {
    super(message);
    this.name = "DoczError";
  }
}

/** Where a document lives: its repository and the config in effect. */
export interface DoczScope {
  root: string;
  /** --config, or empty for the root's own .docz.yaml. */
  config: string;
}

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

/** The docz binary: DOCZ_REVIEW_DOCZ, else `docz` on PATH, else null. */
export function resolveDocz(
  env: Record<string, string | undefined>,
  which: (cmd: string) => string | null = (cmd) =>
    Bun.which(cmd, { PATH: env.PATH ?? "" }),
): string | null {
  const set = env.DOCZ_REVIEW_DOCZ;
  return set !== undefined && set !== "" ? set : which("docz");
}

/** One document's facts for a round, from bytes both commands agreed on. */
export interface Snapshot {
  inspect: DoczInspect;
  findings: Finding[];
}

const TIMEOUT_MS = 60_000;

export class Docz {
  private checked: Promise<string> | undefined;

  constructor(readonly bin: string) {}

  /** Runs docz; a spawn failure or a timeout is a DoczError. */
  run(args: readonly string[], cwd?: string): Promise<RunResult> {
    return new Promise((resolve, reject) => {
      execFile(
        this.bin,
        args,
        {
          cwd,
          maxBuffer: MAX_OUTPUT_BYTES,
          timeout: TIMEOUT_MS,
          encoding: "utf8",
        },
        (err, stdout, stderr) => {
          if (err === null) {
            resolve({ code: 0, stdout, stderr });
            return;
          }
          const code = (err as { code?: unknown }).code;
          if (typeof code === "number" && !err.killed) {
            resolve({ code, stdout, stderr });
            return;
          }
          reject(
            new DoczError("spawn", `could not run ${this.bin}: ${err.message}`),
          );
        },
      );
    });
  }

  /**
   * `docz version`, checked once against the contract this build speaks.
   * Throws VersionError for a docz that predates it.
   */
  version(): Promise<string> {
    this.checked ??= (async () => {
      const res = await this.run(["version"]);
      const version = /^docz (\S+)/.exec(res.stdout)?.[1];
      if (res.code !== 0 || version === undefined) {
        throw new DoczError(
          "exit",
          `docz version: ${firstLine(res.stderr) || "no version printed"}`,
          res.code,
        );
      }
      checkSchemaVersion(SUPPORTED.schemaVersion, version);
      return version;
    })();
    return this.checked;
  }

  /** `docz inspect <target> --format json`. */
  async inspect(target: string, scope: DoczScope): Promise<DoczInspect> {
    const res = await this.run(
      ["inspect", target, "--format", "json", ...scopeArgs(scope)],
      scope.root,
    );
    if (res.code !== 0) {
      throw new DoczError(
        "exit",
        `docz inspect ${target}: ${firstLine(res.stderr) || `exit ${String(res.code)}`}`,
        res.code,
      );
    }
    return parseInspect(res.stdout);
  }

  /**
   * `docz validate <target> --format json`. Exit 1 is findings and still
   * a report; any exit without a report is an error.
   */
  async validate(target: string, scope: DoczScope): Promise<DoczValidate> {
    const res = await this.run(
      ["validate", target, "--format", "json", ...scopeArgs(scope)],
      scope.root,
    );
    if ((res.code !== 0 && res.code !== 1) || res.stdout.trim() === "") {
      throw new DoczError(
        "exit",
        `docz validate ${target}: ${firstLine(res.stderr) || `exit ${String(res.code)}`}`,
        res.code,
      );
    }
    return parseValidate(res.stdout);
  }

  /**
   * inspect and validate over the same bytes. The two are separate
   * processes, so a write between them is caught by comparing their
   * digests; one retry covers an editor mid-save, and a second mismatch
   * is an error rather than a round built from two versions.
   */
  async snapshot(target: string, scope: DoczScope): Promise<Snapshot> {
    for (let attempt = 0; attempt < 2; attempt++) {
      const inspect = await this.inspect(target, scope);
      const out = await this.validate(inspect.path, scope);
      const report = "report" in out ? out.report : out;
      const doc = report.docs?.find((d) => d.path === inspect.path);
      if (doc === undefined) {
        throw new DoczError(
          "exit",
          `docz validate ${target}: no report for ${inspect.path}`,
        );
      }
      if (doc.content_sha256 === inspect.content_sha256) {
        return { inspect, findings: doc.findings ?? [] };
      }
    }
    throw new DoczError(
      "digest",
      `${target} changed while docz read it twice; try again once it is saved`,
    );
  }
}

function scopeArgs(scope: DoczScope): string[] {
  const args = ["--repo-root", scope.root];
  if (scope.config !== "") {
    args.push("--config", scope.config);
  }
  return args;
}

function firstLine(s: string): string {
  return s.trim().split("\n")[0] ?? "";
}
