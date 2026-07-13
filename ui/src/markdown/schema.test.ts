import { describe, expect, it } from "vitest";

import { sanitizeSchema } from "@/markdown/schema";

describe("sanitizeSchema", () => {
  it.each(["script", "iframe", "object", "embed", "style", "form"])(
    "does not allow <%s>",
    (tag) => {
      expect(sanitizeSchema.tagNames).not.toContain(tag);
    },
  );

  it("allows the markdown structural tags", () => {
    for (const tag of ["a", "p", "h2", "table", "pre", "code", "img"]) {
      expect(sanitizeSchema.tagNames).toContain(tag);
    }
  });

  it("allows no event-handler attributes anywhere", () => {
    const allNames = Object.values(sanitizeSchema.attributes ?? {})
      .flat()
      .map((attr) => (typeof attr === "string" ? attr : attr[0]));
    expect(allNames.filter((name) => /^on/i.test(name))).toEqual([]);
  });

  it("restricts href to non-executable protocols", () => {
    const href = sanitizeSchema.protocols?.href ?? [];
    expect(href).toContain("https");
    expect(href).not.toContain("javascript");
    expect(href).not.toContain("data");
  });

  it("restricts img src to http(s)", () => {
    expect(sanitizeSchema.protocols?.src).toEqual(["http", "https"]);
  });

  it("keeps fenced-code language classes for Shiki", () => {
    const codeAttrs = sanitizeSchema.attributes?.code ?? [];
    const className = codeAttrs.find(
      (attr) => Array.isArray(attr) && attr[0] === "className",
    );
    expect(Array.isArray(className)).toBe(true);
    const matcher = (className as [string, RegExp])[1];
    expect(matcher.test("language-go")).toBe(true);
    expect(matcher.test("not-a-language")).toBe(false);
  });

  it("keeps fence meta only within the caption charset", () => {
    const codeAttrs = sanitizeSchema.attributes?.code ?? [];
    const metastring = codeAttrs.find(
      (attr) => Array.isArray(attr) && attr[0] === "metastring",
    );
    expect(Array.isArray(metastring)).toBe(true);
    const matcher = (metastring as [string, RegExp])[1];
    expect(matcher.test("internal/ingest/parse.go")).toBe(true);
    expect(matcher.test("title=Example config (v2)")).toBe(true);
    expect(matcher.test('"><img src=x onerror=alert(1)>')).toBe(false);
    expect(matcher.test("x".repeat(200))).toBe(false);
    expect(matcher.test("")).toBe(false);
  });

  it("clobbers user-authored ids with the user-content prefix", () => {
    expect(sanitizeSchema.clobber).toContain("id");
    expect(sanitizeSchema.clobberPrefix).toBe("user-content-");
  });
});
