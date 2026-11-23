import { describe, expect, it } from "vitest";

import { sourceFiles } from "@/test/source-tree";

/*
 * tokens.css imports the design system's core.css under source(none), so
 * Tailwind generates a package part's classes only when the stylesheet
 * names that part: one
 * `@import "@donaldgifford/design-system/parts/<kind>/<name>.css"` line
 * per part docz imports (DESIGN-0028 §3). A part with no line is not an
 * error anywhere else. It renders unstyled, and jsdom, which loads no
 * stylesheet, cannot see that. This reads the imports and the lines and
 * holds one to the other, in both directions: a line whose part nothing
 * imports is CSS the page ships for nothing.
 *
 * Only components/, blocks/, and shell/ have stylesheets. lib/ and
 * hooks/ render nothing, so they need no line.
 */

const PACKAGE = "@donaldgifford/design-system";
const PART_IMPORT = new RegExp(
  `["']${PACKAGE}/(components|blocks|shell)/([\\w-]+)["']`,
  "g",
);
const PART_LINE = new RegExp(
  `@import\\s+["']${PACKAGE}/parts/(components|blocks|shell)/([\\w-]+)\\.css["']\\s*;`,
  "g",
);

/** The parts a text names, as `<kind>/<name>`, by the given pattern. */
function parts(text: string, pattern: RegExp): string[] {
  return [...text.matchAll(pattern)].map(
    (match) => `${String(match[1])}/${String(match[2])}`,
  );
}

function sorted(values: Iterable<string>): string[] {
  return [...new Set(values)].sort();
}

describe("the design system's parts", () => {
  const modules = sourceFiles(/\.(ts|tsx)$/);
  const stylesheet = sourceFiles(/theme\/tokens\.css$/);
  const imported = sorted(
    modules.flatMap((file) => parts(file.text, PART_IMPORT)),
  );
  const lines = stylesheet.flatMap((file) => parts(file.text, PART_LINE));

  it("reads the modules and the one stylesheet", () => {
    expect(modules.length).toBeGreaterThan(150);
    expect(stylesheet).toHaveLength(1);
  });

  it("has a parts/ line in tokens.css for every part imported under src", () => {
    const missing = imported.filter((part) => !lines.includes(part));
    expect(
      missing.map((part) => `@import "${PACKAGE}/parts/${part}.css";`),
    ).toEqual([]);
  });

  it("has no parts/ line for a part nothing imports", () => {
    expect(lines.filter((part) => !imported.includes(part))).toEqual([]);
  });

  it("lists each part once, in order", () => {
    expect(lines).toEqual(sorted(lines));
  });

  it("reads an import and a line as the same part", () => {
    expect(
      parts(
        `import { Badge } from "${PACKAGE}/components/badge";
         import { Dot, ListRow } from '${PACKAGE}/blocks/list-row';
         import { cn } from "${PACKAGE}/lib/utils";
         import { useTreeOpen } from "${PACKAGE}/hooks/use-tree-open";`,
        PART_IMPORT,
      ),
    ).toEqual(["components/badge", "blocks/list-row"]);
    expect(
      parts(
        `@import "${PACKAGE}/core.css" source(none);
         @import "${PACKAGE}/parts/components/badge.css";
         @import '${PACKAGE}/parts/shell/sidebar-tree.css' ;`,
        PART_LINE,
      ),
    ).toEqual(["components/badge", "shell/sidebar-tree"]);
  });
});
