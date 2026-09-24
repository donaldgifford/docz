import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  DEFAULT_MERMAID_LAYOUT,
  mermaidLayout,
  parseMermaidLayout,
} from "@/lib/mermaidLayout";

/*
 * The both-ends rule (IMPL-0006 Phase 5): server/serve.ts whitelists
 * DOCZ_MERMAID_LAYOUT before injecting it, and this module re-validates
 * whatever it reads. The value goes straight into `mermaid.initialize`,
 * so an unvalidated string would be config injection on the one library
 * that renders untrusted document text.
 */

beforeEach(() => {
  delete window.__DOCZ_CONFIG__;
});

afterEach(() => {
  delete window.__DOCZ_CONFIG__;
  vi.unstubAllEnvs();
});

describe("parseMermaidLayout", () => {
  it.each([
    ["dagre", "dagre"],
    ["elk", "elk"],
    [" ELK ", "elk"],
    ["Dagre", "dagre"],
  ])("accepts %j as %j", (raw, expected) => {
    expect(parseMermaidLayout(raw)).toBe(expected);
  });

  it.each([
    [""],
    ["   "],
    ["cytoscape"],
    ["elk,dagre"],
    ["__proto__"],
    ["elk</script><script>alert(1)</script>"],
    ['"><img src=x onerror=alert(1)>"'],
  ])("rejects %j", (raw) => {
    expect(parseMermaidLayout(raw)).toBeUndefined();
  });

  it.each([[undefined], [null], [42], [["elk"]], [{ layout: "elk" }]])(
    "rejects the non-string %j",
    (raw) => {
      expect(parseMermaidLayout(raw)).toBeUndefined();
    },
  );
});

describe("mermaidLayout", () => {
  it("defaults to elk with no config at all", () => {
    expect(mermaidLayout()).toBe("elk");
    expect(DEFAULT_MERMAID_LAYOUT).toBe("elk");
  });

  it("takes the injected runtime value", () => {
    window.__DOCZ_CONFIG__ = { mermaidLayout: "dagre" };
    expect(mermaidLayout()).toBe("dagre");
  });

  it("prefers the injected value over the build-time one", () => {
    vi.stubEnv("VITE_MERMAID_LAYOUT", "dagre");
    window.__DOCZ_CONFIG__ = { mermaidLayout: "elk" };
    expect(mermaidLayout()).toBe("elk");
  });

  it("falls back to the build-time value when nothing is injected", () => {
    vi.stubEnv("VITE_MERMAID_LAYOUT", "dagre");
    expect(mermaidLayout()).toBe("dagre");
  });

  it("falls through a hostile injected value rather than honoring it", () => {
    // Unlike nav pins, an injected value that fails validation is not
    // authoritative: there is no "chose nothing" case to respect, so a
    // bad value means something is wrong and the next source wins.
    vi.stubEnv("VITE_MERMAID_LAYOUT", "dagre");
    window.__DOCZ_CONFIG__ = {
      mermaidLayout: "elk</script><script>alert(1)</script>",
    };
    expect(mermaidLayout()).toBe("dagre");
  });

  it("falls all the way back to elk when both sources are junk", () => {
    vi.stubEnv("VITE_MERMAID_LAYOUT", "cytoscape");
    window.__DOCZ_CONFIG__ = { mermaidLayout: { toString: () => "dagre" } };
    expect(mermaidLayout()).toBe("elk");
  });
});
