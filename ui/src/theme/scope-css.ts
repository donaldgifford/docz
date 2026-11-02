/*
 * Scopes pkg/theme's whole-page stylesheets to one element (DESIGN-0026
 * §5), so the reader can embed an exported document's fragment without
 * its `body`, `h1`, or `table` rules reaching the shell around it.
 *
 * pkg/theme writes two shapes, and this reads only those two: flat rules
 * (`selectors { declarations }`) and rules inside `@media`. Anything else,
 * or a selector rooted at `html`, `body`, or `:root` in a form the table
 * below does not name, is an error, so a new shape in pkg/theme fails the
 * generator instead of leaking into the shell.
 *
 *   :root[data-theme=x][data-mode=dark]  →  :root[data-mode=dark] S[data-theme=x]
 *   :root[data-theme=x]:not([data-mode]) →  :root:not([data-mode]) S[data-theme=x]
 *   :root[data-theme=x], html[data-theme=x] → S[data-theme=x]
 *   :root, html, body                    →  S
 *   html.js X, html:not(.js) X           →  S.js X, S:not(.js) X
 *   anything else X                      →  S X
 *
 * where S is the scope selector. Comments are dropped.
 */

const THEME = /^(?::root|html)(\[data-theme="[a-z0-9-]+"\])(.*)$/;

/** One selector, rewritten under scope. */
export function scopeSelector(selector: string, scope: string): string {
  const sel = selector.trim();

  const theme = THEME.exec(sel);
  if (theme !== null) {
    const [, attr = "", rest = ""] = theme;
    if (rest === "") return `${scope}${attr}`;
    if (rest === '[data-mode="dark"]')
      return `:root[data-mode="dark"] ${scope}${attr}`;
    if (rest === ":not([data-mode])")
      return `:root:not([data-mode]) ${scope}${attr}`;
    throw new Error(`scope-css: unsupported theme selector: ${sel}`);
  }

  if (sel === ":root" || sel === "html" || sel === "body") return scope;

  const js = /^html(\.js|:not\(\.js\))(\s.*)$/.exec(sel);
  if (js !== null) return `${scope}${js[1] ?? ""}${js[2] ?? ""}`;

  if (/^(?::root|html|body)\b/.test(sel)) {
    throw new Error(`scope-css: unsupported root selector: ${sel}`);
  }

  return `${scope} ${sel}`;
}

function scopeList(prelude: string, scope: string): string {
  return prelude
    .split(",")
    .map((s) => scopeSelector(s, scope))
    .join(",\n");
}

function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, "");
}

/** The index of the `}` closing a declaration block that opens at i. */
function blockEnd(css: string, i: number): number {
  let quote = "";
  for (let j = i; j < css.length; j++) {
    const c = css[j];
    if (quote !== "") {
      if (c === "\\") j++;
      else if (c === quote) quote = "";
    } else if (c === '"' || c === "'") {
      quote = c;
    } else if (c === "{") {
      throw new Error("scope-css: a nested block inside a rule");
    } else if (c === "}") {
      return j;
    }
  }
  throw new Error("scope-css: an unclosed block");
}

/** Rewrites a run of flat rules; stops at an unmatched `}` or the end. */
function rules(
  css: string,
  start: number,
  scope: string,
  inMedia: boolean,
): { out: string; end: number } {
  let out = "";
  let i = start;
  for (;;) {
    while (i < css.length && /\s/.test(css[i] ?? "")) i++;
    if (i >= css.length) {
      if (inMedia) throw new Error("scope-css: an unclosed @media");
      return { out, end: i };
    }
    if (css[i] === "}") {
      if (!inMedia) throw new Error("scope-css: an unmatched }");
      return { out, end: i };
    }
    const open = css.indexOf("{", i);
    if (open < 0) throw new Error("scope-css: a rule with no block");
    const prelude = css.slice(i, open).trim();

    if (prelude.startsWith("@")) {
      if (inMedia || !prelude.startsWith("@media ")) {
        throw new Error(`scope-css: unsupported at-rule: ${prelude}`);
      }
      const inner = rules(css, open + 1, scope, true);
      out += `${prelude} {\n${inner.out}}\n`;
      i = inner.end + 1;
      continue;
    }

    const close = blockEnd(css, open + 1);
    const body = css.slice(open + 1, close).trim();
    out += `${scopeList(prelude, scope)} {\n  ${body}\n}\n`;
    i = close + 1;
  }
}

/** Rewrites a whole stylesheet so every rule applies inside scope only. */
export function scopeCss(css: string, scope: string): string {
  return rules(stripComments(css), 0, scope, false).out;
}
