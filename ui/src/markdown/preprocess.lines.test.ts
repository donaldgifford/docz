/// <reference types="node" />
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";
import { visit } from "unist-util-visit";
import { describe, expect, it } from "vitest";

import { preprocessDoczMarkdown } from "@/markdown/preprocess";

// The parity fixtures are real docz documents: frontmatter, a ToC
// block, markers, and every built-in type.
const FIXTURES = resolve(process.cwd(), "../test/parity/fixtures");

function documents(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      return documents(path);
    }
    return /^\d+-.*\.md$/.test(name) ? [path] : [];
  });
}

describe("preprocessDoczMarkdown preserveLines", () => {
  it("blanks frontmatter, the ToC block, and the leading h1 in place", () => {
    const raw = [
      "---",
      "id: IMPL-0001",
      "---",
      "",
      "# IMPL 0001",
      "",
      "<!--toc:start-->",
      "- [Phase 0](#phase-0)",
      "<!--toc:end-->",
      "",
      "## Phase 0",
    ].join("\n");

    const out = preprocessDoczMarkdown(raw, {
      preserveLines: true,
      stripLeadingH1: true,
    });

    expect(out.split("\n")).toHaveLength(raw.split("\n").length);
    expect(out).toBe("\n\n\n\n\n\n\n\n\n\n## Phase 0");
  });

  const paths = documents(FIXTURES);

  it("finds the fixture documents", () => {
    expect(paths.length).toBeGreaterThan(10);
  });

  it.each(paths.map((p) => [p.slice(FIXTURES.length + 1), p]))(
    "every block in %s starts on its document line",
    (_name, path) => {
      const raw = readFileSync(path, "utf8");
      const out = preprocessDoczMarkdown(raw, { preserveLines: true });
      const source = raw.split("\n");
      const kept = out.split("\n");
      expect(kept).toHaveLength(source.length);

      const tree = unified().use(remarkParse).use(remarkGfm).parse(out);
      let blocks = 0;
      visit(tree, (node) => {
        if (node.type === "root" || node.position === undefined) {
          return;
        }
        const line = node.position.start.line;
        // The line the parser names is the document's own line, untouched.
        expect(kept[line - 1]).toBe(source[line - 1]);
        expect(kept[line - 1]?.trim()).not.toBe("");
        blocks += 1;
      });
      expect(blocks).toBeGreaterThan(0);
    },
  );
});
