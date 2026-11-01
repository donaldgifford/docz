/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { scopeCss, scopeSelector } from "@/theme/scope-css";

const S = ".docz-page";

describe("scopeSelector", () => {
  it.each([
    [':root[data-theme="paper"]', '.docz-page[data-theme="paper"]'],
    ['html[data-theme="charcoal"]', '.docz-page[data-theme="charcoal"]'],
    [
      ':root[data-theme="paper"][data-mode="dark"]',
      ':root[data-mode="dark"] .docz-page[data-theme="paper"]',
    ],
    [
      ':root[data-theme="paper"]:not([data-mode])',
      ':root:not([data-mode]) .docz-page[data-theme="paper"]',
    ],
    [":root", ".docz-page"],
    ["html", ".docz-page"],
    ["body", ".docz-page"],
    ["html.js section.panel", ".docz-page.js section.panel"],
    ["html:not(.js) .filters", ".docz-page:not(.js) .filters"],
    ["h1", ".docz-page h1"],
    ["::selection", ".docz-page ::selection"],
    ["details.expand > .body", ".docz-page details.expand > .body"],
  ])("%s → %s", (input, want) => {
    expect(scopeSelector(input, S)).toBe(want);
  });

  it.each([
    'html[data-theme="paper"] h1',
    "html > body",
    "body.dark",
    ":root.js",
  ])("refuses %s", (input) => {
    expect(() => scopeSelector(input, S)).toThrow(/unsupported/);
  });
});

describe("scopeCss", () => {
  it("scopes flat rules and rules inside @media, dropping comments", () => {
    const css = `/* c */
html,
body { margin: 0; }
@media (prefers-color-scheme: dark) {
  :root[data-theme="paper"]:not([data-mode]) { --x: 1; }
}
a::after { content: "}{"; }
`;
    expect(scopeCss(css, S)).toBe(`.docz-page,
.docz-page {
  margin: 0;
}
@media (prefers-color-scheme: dark) {
:root:not([data-mode]) .docz-page[data-theme="paper"] {
  --x: 1;
}
}
.docz-page a::after {
  content: "}{";
}
`);
  });

  it.each([
    ["@supports (display: grid) { a { b: c; } }", /unsupported at-rule/],
    ["@media print { @media screen { a { b: c; } } }", /unsupported at-rule/],
    ["a { b { c: d; } }", /nested block/],
    ["a { b: c;", /unclosed block/],
    ["}", /unmatched/],
  ])("refuses %s", (css, err) => {
    expect(() => scopeCss(css, S)).toThrow(err);
  });

  it("reads every pkg/theme stylesheet", () => {
    const dir = resolve(process.cwd(), "../pkg/theme");
    for (const file of [
      "base.css",
      "components.css",
      "tokens/paper.css",
      "tokens/classic.css",
      "tokens/charcoal.css",
    ]) {
      const out = scopeCss(readFileSync(resolve(dir, file), "utf8"), S);
      // Every rule's selector list is scoped: nothing left can match an
      // element outside the scope.
      for (const block of out.split("}\n")) {
        const prelude = block.slice(0, block.indexOf("{")).trim();
        if (prelude === "" || prelude.startsWith("@media")) continue;
        for (const sel of prelude.split(",\n")) {
          expect(sel, `${file}: ${sel}`).toMatch(
            /^(\.docz-page|:root(\[data-mode="dark"\]|:not\(\[data-mode\]\)) \.docz-page)/,
          );
        }
      }
    }
  });
});
