import { describe, expect, it } from "vitest";

import { sourceFiles } from "@/test/source-tree";

/*
 * Two vocabularies, one place each (DESIGN-0028 §3, IMPL-0033 Phase 9).
 * The chrome is written in the design system's names (bg-card,
 * text-muted-foreground, border). pkg/theme's names (bg-bg-raised,
 * text-fg-prose, text-t-rfc, bg-brand) are for markup inside a
 * .docz-page, where page.css gives each name the document theme's
 * value. tokens.css registers them `inline` with no value of their own,
 * so one written in chrome resolves to nothing: no background, or text
 * in the inherited colour. Nothing else fails on that, and jsdom cannot
 * see it. This does.
 *
 * It reads the files as text: a class name in a string or a template is
 * the same mistake. Tests are not scanned, since they name utilities in
 * order to assert on them.
 */

/** Where a document-scoped name may be written. */
const DOCUMENT_SCOPE = [
  /^markdown\//,
  /^components\/doc-pane\.tsx$/,
  /^components\/doc-surface\.tsx$/,
  // The review page's document view: the regions wrapped around the
  // rendered document. The rest of src/review is chrome.
  /^review\/regions\.tsx$/,
];

/** Names that are the package's utilities, so docz registers no entry. */
const PACKAGE_OWNED = ["accent", "code-bg", "border-strong", "st-superseded"];

const PREFIXES = [
  "bg",
  "text",
  "border",
  "outline",
  "ring",
  "ring-offset",
  "fill",
  "stroke",
  "decoration",
  "divide",
  "from",
  "via",
  "to",
  "caret",
  "shadow",
  "placeholder",
];

const tokensCss =
  sourceFiles(/\.css$/).find((file) => file.path === "theme/tokens.css")
    ?.text ?? "";

/** The `@theme inline` block's entries: utility name and value. */
function registered(css: string): [name: string, value: string][] {
  const block = /@theme inline \{([^}]*)\}/.exec(css)?.[1] ?? "";
  return [...block.matchAll(/--color-([a-z0-9-]+):\s*([^;]+);/g)].map(
    (match) => [match[1] ?? "", (match[2] ?? "").trim()],
  );
}

function utilityPattern(names: readonly string[]): RegExp {
  return new RegExp(
    `(?<![\\w-])(?:${PREFIXES.join("|")})-(?:${names.join("|")})(?![\\w-])`,
    "g",
  );
}

describe("the document-scoped utility names", () => {
  const entries = registered(tokensCss);
  const names = entries.map(([name]) => name);
  const pattern = utilityPattern(names);
  const chrome = sourceFiles(/\.(ts|tsx|html)$/).filter(
    (file) =>
      !/\.test\.tsx?$/.test(file.path) &&
      !file.path.startsWith("test/") &&
      !DOCUMENT_SCOPE.some((scope) => scope.test(file.path)),
  );

  it("reads the names from tokens.css", () => {
    expect(names.length).toBeGreaterThanOrEqual(45);
    expect(names).toEqual(
      expect.arrayContaining(["bg-raised", "fg-prose", "t-rfc", "brand"]),
    );
  });

  it("scans the chrome", () => {
    expect(chrome.length).toBeGreaterThan(60);
    expect(chrome.some((file) => file.path === "app/AppShell.tsx")).toBe(true);
    expect(chrome.some((file) => file.path === "review/app.tsx")).toBe(true);
    expect(chrome.some((file) => file.path.startsWith("markdown/"))).toBe(
      false,
    );
  });

  it("are written nowhere in chrome: use the design system's names", () => {
    const found = chrome.flatMap((file) =>
      file.text
        .split("\n")
        .flatMap((line, index) =>
          [...line.matchAll(pattern)].map(
            (match) => `${file.path}:${String(index + 1)} ${match[0]}`,
          ),
        ),
    );
    expect(found).toEqual([]);
  });

  it("catches a docz name and leaves the package's", () => {
    const hits = (text: string) =>
      [...text.matchAll(pattern)].map((match) => match[0]);
    expect(
      hits(
        "bg-bg-raised hover:text-fg-muted border-border-default text-t-rfc bg-brand text-on-brand ring-hash-3",
      ),
    ).toEqual([
      "bg-bg-raised",
      "text-fg-muted",
      "border-border-default",
      "text-t-rfc",
      "bg-brand",
      "text-on-brand",
      "ring-hash-3",
    ]);
    expect(
      hits(
        "bg-card text-muted-foreground border-border border-border-strong bg-code-bg text-primary border-t border-t-2 var(--color-t-rfc) text-(--color-bad)",
      ),
    ).toEqual([]);
  });

  /*
   * A name with no value at :root is what keeps it out of the chrome.
   * Each entry is its own value (or, for brand, pkg/theme's accent
   * name), so Tailwind compiles the utility to the variable and
   * declares nothing. A package token as the value would be the alias
   * Phase 9 removed.
   */
  it("each stand for a pkg/theme variable, never a package token", () => {
    for (const [name, value] of entries) {
      const own = name
        .replace(/^on-brand$/, "on-accent")
        .replace(/^brand/, "accent");
      expect(value, `--color-${name}`).toBe(`var(--color-${own})`);
    }
  });

  it("leave the package's own utility names alone", () => {
    for (const name of PACKAGE_OWNED) {
      expect(names, name).not.toContain(name);
    }
  });

  it("have no alias stylesheet beside them", () => {
    expect(tokensCss).not.toContain("shell.css");
    expect(
      sourceFiles(/\.css$/).some((file) => file.path === "theme/shell.css"),
    ).toBe(false);
  });
});
