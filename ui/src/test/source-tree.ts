/// <reference types="node" />
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";

/*
 * The tests that hold a rule over the whole of src/ by reading it as
 * text (the accent utility, the parts/ lines) walk it through here, so
 * each one skips the same things: generated output, which docz does not
 * write, and CSS comments, which explain a rule by naming what it bans.
 */

export const SRC = resolve(process.cwd(), "src");

export interface SourceFile {
  /** The path below src/, with forward slashes. */
  readonly path: string;
  /** The file's text, with comments removed from a stylesheet. */
  readonly text: string;
}

function walk(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      return entry.name === "__generated__" ? [] : walk(path);
    }
    return [path];
  });
}

/** Every file under src/ whose name matches, outside `__generated__`. */
export function sourceFiles(matching: RegExp): SourceFile[] {
  return walk(SRC)
    .filter((path) => matching.test(path))
    .map((path) => {
      const text = readFileSync(path, "utf8");
      return {
        path: relative(SRC, path).split("\\").join("/"),
        text: path.endsWith(".css")
          ? text.replace(/\/\*[\s\S]*?\*\//g, "")
          : text,
      };
    });
}
