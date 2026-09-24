import { describe, expect, it } from "vitest";

import { resolveRelativeLinks } from "@/markdown/relative-links";

import type { Element, Root } from "hast";

/*
 * Pure-transform coverage: hast in, hrefs rewritten (or provably not).
 * The pipeline-level behavior — markdown through renderMarkdown with a
 * base — is pinned in processor tests once the option is threaded.
 */

const BY_PATH = new Map([
  ["docs/adr/0013-scoped-test-ids.md", "/acme/mods/adr/ADR-0013"],
  ["docs/rfc/RFC-0001-first.md", "/acme/mods/rfc/RFC-0001"],
  ["docs/rfc/RFC-0002-second.md", "/acme/mods/rfc/RFC-0002"],
  ["docs/design/0001-a design.md", "/acme/mods/design/DESIGN-0001"],
]);

/** The RFC-0001 doc is the rendering source for most cases. */
const BASE = "docs/rfc/RFC-0001-first.md";

function anchor(href: string): Element {
  return {
    type: "element",
    tagName: "a",
    properties: { href },
    children: [{ type: "text", value: "link" }],
  };
}

function transformed(href: string, base = BASE): Element {
  const node = anchor(href);
  const tree: Root = { type: "root", children: [node] };
  resolveRelativeLinks(tree, { base, byPath: BY_PATH });
  return node;
}

describe("resolveRelativeLinks", () => {
  it("resolves ../type/file.md against the source's directory", () => {
    const node = transformed("../adr/0013-scoped-test-ids.md");
    expect(node.properties.href).toBe("/acme/mods/adr/ADR-0013");
    expect(node.properties.dataXref).toBe(true);
  });

  it("resolves a bare sibling filename", () => {
    const node = transformed("RFC-0002-second.md");
    expect(node.properties.href).toBe("/acme/mods/rfc/RFC-0002");
  });

  it("resolves an explicit ./ sibling", () => {
    const node = transformed("./RFC-0002-second.md");
    expect(node.properties.href).toBe("/acme/mods/rfc/RFC-0002");
  });

  it("reattaches the #fragment on a hit", () => {
    const node = transformed("../adr/0013-scoped-test-ids.md#context");
    expect(node.properties.href).toBe("/acme/mods/adr/ADR-0013#context");
    expect(node.properties.dataXref).toBe(true);
  });

  it("decodes percent-encoding before the exact-path lookup", () => {
    const node = transformed("../design/0001-a%20design.md");
    expect(node.properties.href).toBe("/acme/mods/design/DESIGN-0001");
  });

  it.each([
    ["a miss at the wrong depth", "0013-scoped-test-ids.md"],
    ["a file the repo doesn't serve", "../adr/9999-nope.md"],
    ["traversal past the repo root", "../../../../etc/passwd"],
    ["percent-encoded traversal", "%2e%2e/%2e%2e/%2e%2e/secret.md"],
    ["malformed percent-encoding", "RFC-0002-second.md%"],
    ["an absolute URL", "https://example.com/docs/rfc/RFC-0002-second.md"],
    ["a protocol-relative URL", "//example.com/RFC-0002-second.md"],
    ["a root-absolute path", "/docs/rfc/RFC-0002-second.md"],
    ["a fragment-only href", "#context"],
    ["a mailto: href", "mailto:docs@example.com"],
  ])("leaves %s untouched", (_case, href) => {
    const node = transformed(href);
    expect(node.properties.href).toBe(href);
    expect(node.properties.dataXref).toBeUndefined();
  });

  it("treats .. from a repo-root source as escaping (fails closed)", () => {
    const node = transformed("../docs/rfc/RFC-0002-second.md", "CHANGELOG.md");
    expect(node.properties.href).toBe("../docs/rfc/RFC-0002-second.md");
  });

  it("resolves docs-dir-relative links from a repo-root source", () => {
    const node = transformed("docs/rfc/RFC-0002-second.md", "CHANGELOG.md");
    expect(node.properties.href).toBe("/acme/mods/rfc/RFC-0002");
  });
});
