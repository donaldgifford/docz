/// <reference types="node" />
import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { validateWidget, type WidgetName } from "./validate";

/*
 * pkg/export/html/testdata/widgets is the shared corpus (IMPL-0030 Open
 * Question 9): the Go tests write each block's golden with -update, and
 * this reads the same files, so the port and widgets.go cannot drift
 * apart. Each block is read as if its fence opened on line 1.
 */
const dir = resolve(process.cwd(), "../pkg/export/html/testdata/widgets");
const FIRST = 2;

const blocks = readdirSync(dir)
  .filter((f) => f.endsWith(".yaml"))
  .map((f) => f.slice(0, -".yaml".length));

function golden(base: string, kind: "json" | "errors"): string | undefined {
  try {
    return readFileSync(resolve(dir, `${base}.${kind}.golden`), "utf8");
  } catch {
    return undefined;
  }
}

function lineOf(error: string): string {
  return /^line \d+/.exec(error)?.[0] ?? error;
}

describe("the widget corpus", () => {
  it("finds the blocks", () => {
    expect(blocks.length).toBeGreaterThanOrEqual(30);
  });

  it.each(blocks)("%s", (base) => {
    const widget = base.slice(0, base.indexOf("-")) as WidgetName;
    const source = readFileSync(resolve(dir, `${base}.yaml`), "utf8");
    const got = validateWidget(widget, source, FIRST);
    const json = golden(base, "json");

    if (json !== undefined) {
      expect(got.ok, JSON.stringify(got)).toBe(true);
      if (got.ok) {
        expect(JSON.stringify(got.widget.data)).toBe(json.trimEnd());
      }
      return;
    }

    const want = (golden(base, "errors") ?? "").trimEnd().split("\n");
    expect(got.ok).toBe(false);
    if (got.ok) {
      return;
    }
    // A block the parser refuses carries the parser's own wording, so
    // only its line is held to Go's (testdata/widgets/README.md).
    if (base.startsWith(`${widget}-parse-`)) {
      expect(got.errors.map(lineOf)).toEqual(want.map(lineOf));
    } else {
      expect(got.errors).toEqual(want);
    }
  });
});
