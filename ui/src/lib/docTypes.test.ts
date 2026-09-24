import { describe, expect, it } from "vitest";

import { resolveDocType } from "@/lib/docTypes";

import type { DocType } from "@/api/__generated__/docz-api.schemas";

// The real api marshals empty Go slices as JSON null even though the
// generated type says string[] (see src/lib/wire.ts) — regression for
// the reader crash "can't access property includes, e.aliases is null".
const WIRE_NULL_TYPE: DocType = {
  name: "design",
  dir: "design",
  id_prefix: "DESIGN",
  plural_label: "Designs",
  statuses: ["Draft", "Approved"],
  aliases: null as unknown as string[],
};

const ALIASED_TYPE: DocType = {
  name: "investigation",
  dir: "investigation",
  id_prefix: "INV",
  plural_label: "Investigations",
  statuses: ["Open"],
  aliases: ["inv"],
};

describe("resolveDocType", () => {
  it("survives wire-null aliases and still resolves name/id_prefix", () => {
    const types = [WIRE_NULL_TYPE, ALIASED_TYPE];
    expect(resolveDocType(types, "design")?.name).toBe("design");
    expect(resolveDocType(types, "DESIGN")?.name).toBe("design");
    expect(resolveDocType(types, "nope")).toBeUndefined();
  });

  it("resolves aliases case-insensitively", () => {
    expect(resolveDocType([ALIASED_TYPE], "INV")?.name).toBe("investigation");
    expect(resolveDocType([ALIASED_TYPE], "inv")?.name).toBe("investigation");
  });
});
