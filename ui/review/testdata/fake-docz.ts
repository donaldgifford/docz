// A fake docz binary for docz-review's tests: a shell script that runs
// fake-docz-main.ts with this fake's config, so execFile sees a real
// executable and a real exit code.
import { chmodSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { Finding } from "@/cli/__generated__/validate";

export interface FakeResponse {
  code?: number;
  stdout?: string;
  stderr?: string;
}

export type FakeConfig = Record<string, FakeResponse[]>;

export interface FakeCall {
  args: string[];
  cwd: string;
}

export interface FakeDocz {
  bin: string;
  /** Replaces the responses; the per-command counters keep counting. */
  set(config: FakeConfig): void;
  calls(): FakeCall[];
}

export function fakeDocz(config: FakeConfig): FakeDocz {
  const dir = mkdtempSync(join(tmpdir(), "fake-docz-"));
  const cfg = join(dir, "config.json");
  const bin = join(dir, "docz");
  writeFileSync(cfg, JSON.stringify(config));
  writeFileSync(
    bin,
    `#!/bin/sh\nFAKE_DOCZ_CONFIG='${cfg}' exec '${process.execPath}' '${join(import.meta.dir, "fake-docz-main.ts")}' "$@"\n`,
  );
  chmodSync(bin, 0o755);
  return {
    bin,
    set(next) {
      writeFileSync(cfg, JSON.stringify(next));
    },
    calls() {
      try {
        return readFileSync(`${cfg}.log`, "utf8")
          .trim()
          .split("\n")
          .map((l) => JSON.parse(l) as FakeCall);
      } catch {
        return [];
      }
    },
  };
}

export const version = (v = "v2.2.0"): FakeResponse[] => [
  { stdout: `docz ${v} (commit: abc1234)\n` },
];

/** A validate report for one document. */
export function validateOutput(
  inspect: Pick<DoczInspect, "type" | "path" | "content_sha256">,
  findings: Finding[] = [],
): FakeResponse {
  const errors = findings.filter((f) => f.severity === "error").length;
  return {
    code: findings.length > 0 ? 1 : 0,
    stdout: JSON.stringify({
      schema_version: 1,
      docs: [
        {
          type: inspect.type,
          path: inspect.path,
          schema: inspect.type,
          content_sha256: inspect.content_sha256,
          findings,
        },
      ],
      errors,
      warnings: findings.length - errors,
    }),
  };
}
