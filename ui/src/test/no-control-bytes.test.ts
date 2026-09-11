/// <reference types="node" />
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/*
 * Source files must be text.
 *
 * A raw control byte inside a string literal runs correctly, typechecks,
 * lints, formats, and passes every other gate in this repo — and makes
 * git classify the whole file as BINARY. It then has no diff, no blame,
 * and no three-way merge, which is how `useActiveHeading.ts` shipped in
 * v0.7.0 with a literal 0x00 as its separator constant. Nothing else
 * here would have caught it. Write control characters as escapes.
 *
 * Tab, newline, and carriage return are the ones git tolerates, so they
 * are the ones allowed.
 */

const ROOTS = ["src", "e2e", "server", "scripts"];
const EXTENSIONS = [".ts", ".tsx", ".css", ".js", ".mjs"];
// orval output, never hand-edited and not committed.
const SKIP_DIRS = new Set(["__generated__", "node_modules"]);

// eslint-disable-next-line no-control-regex
const FORBIDDEN = /[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/;

function sourceFiles(dir: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (SKIP_DIRS.has(entry)) {
      continue;
    }
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      found.push(...sourceFiles(path));
    } else if (EXTENSIONS.some((ext) => entry.endsWith(ext))) {
      found.push(path);
    }
  }
  return found;
}

describe("source files", () => {
  const files = ROOTS.flatMap((root) => sourceFiles(root));

  it("finds files to check", () => {
    // A broken walk would make the real assertion vacuously pass.
    expect(files.length).toBeGreaterThan(100);
  });

  it.each(files)("%s carries no raw control bytes", (path) => {
    const lines = readFileSync(path, "utf8").split("\n");
    const offenders = lines.flatMap((line, i) => {
      const match = FORBIDDEN.exec(line);
      if (match === null) {
        return [];
      }
      const code = match[0].codePointAt(0) ?? 0;
      return [`${path}:${String(i + 1)} contains U+${hex(code)}`];
    });
    expect(offenders).toEqual([]);
  });
});

function hex(code: number): string {
  return code.toString(16).toUpperCase().padStart(4, "0");
}
