/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  colorToHex,
  oklchToHex,
  referenceName,
  resolveToken,
  tokenHex,
  TokenError,
  type TokenSource,
} from "./oklch";

function sourceOf(tokens: Record<string, string>): TokenSource {
  return (name) => tokens[name];
}

describe("referenceName", () => {
  it.each([
    ["{slate-98}", "slate-98"],
    ["var(--slate-98)", "slate-98"],
    ["  var( --primary )  ", "primary"],
  ])("reads %s", (value, name) => {
    expect(referenceName(value)).toBe(name);
  });

  it.each(["oklch(1 0 0)", "#ffffff", "black", "{a} {b}", "var(--a, red)"])(
    "treats %s as a literal",
    (value) => {
      expect(referenceName(value)).toBeNull();
    },
  );
});

describe("resolveToken", () => {
  it("follows a chain of references to the literal", () => {
    const source = sourceOf({
      "sidebar-ring": "{ring}",
      ring: "var(--primary)",
      primary: "{blue-50}",
      "blue-50": "oklch(0.5013 0.1435 254.06)",
    });
    expect(resolveToken("sidebar-ring", source)).toBe(
      "oklch(0.5013 0.1435 254.06)",
    );
  });

  it("fails on a cycle and names it", () => {
    const source = sourceOf({ a: "{b}", b: "{c}", c: "{a}" });
    expect(() => resolveToken("a", source)).toThrow(TokenError);
    expect(() => resolveToken("a", source)).toThrow(
      "token cycle: a -> b -> c -> a",
    );
  });

  it("fails on a token that points at itself", () => {
    expect(() => resolveToken("a", sourceOf({ a: "var(--a)" }))).toThrow(
      "token cycle: a -> a",
    );
  });

  it("fails on an unknown name", () => {
    expect(() => resolveToken("nope", sourceOf({}))).toThrow(
      "unknown token: nope",
    );
  });

  it("names the chain that led to an unknown name", () => {
    const source = sourceOf({ card: "{gray-100}" });
    expect(() => resolveToken("card", source)).toThrow(
      "unknown token: gray-100 (from card)",
    );
  });
});

describe("colorToHex", () => {
  it.each([
    ["oklch(1 0 0)", "#ffffff"],
    ["oklch(0 0 0)", "#000000"],
    ["oklch(100% 0 0)", "#ffffff"],
    ["#FFAA00", "#ffaa00"],
    ["#abc", "#aabbcc"],
  ])("converts %s", (value, hex) => {
    expect(colorToHex(value)).toBe(hex);
  });

  it("refuses what is not a colour it knows", () => {
    expect(() => colorToHex("black")).toThrow(TokenError);
    expect(() => oklchToHex("rgb(0 0 0)")).toThrow("not an oklch() colour");
  });
});

/*
 * The eleven tokens INV-0036's spike mapped (Observation 10): each
 * package token converts to the hex docz's shell carried before the
 * package arrived, in both modes. They are read from the installed
 * package's tokens.json, so a bump that moves one fails here.
 */
interface TokenFile {
  light: Record<string, { $value: string } | undefined>;
  dark: Record<string, { $value: string } | undefined>;
}

const tokenFile = JSON.parse(
  readFileSync(
    resolve(
      process.cwd(),
      "node_modules/@donaldgifford/design-system/tokens.json",
    ),
    "utf8",
  ),
) as TokenFile;

const SPIKE_VALUES: [name: string, light: string, dark: string][] = [
  ["background", "#f6f7f9", "#17191c"],
  ["card", "#ffffff", "#202328"],
  ["secondary", "#edeff2", "#262a31"],
  ["border", "#d9dde3", "#32373e"],
  ["foreground", "#1a1e23", "#e8eaed"],
  ["muted-foreground", "#515b67", "#9aa2ac"],
  ["primary", "#1963b3", "#61a8f5"],
  ["success", "#196629", "#40ba52"],
  ["warning", "#835107", "#d99f21"],
  ["destructive", "#a7241b", "#f7756e"],
  ["info", "#2757b0", "#7aa7ff"],
];

describe("the package's tokens.json", () => {
  it.each(SPIKE_VALUES)("%s is %s light and %s dark", (name, light, dark) => {
    expect(tokenHex(name, (n) => tokenFile.light[n]?.$value)).toBe(light);
    expect(tokenHex(name, (n) => tokenFile.dark[n]?.$value)).toBe(dark);
  });
});
