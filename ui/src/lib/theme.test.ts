import { afterEach, describe, expect, it, vi } from "vitest";

import { defaultTheme, parseTheme, storedTheme } from "@/lib/theme";

afterEach(() => {
  delete window.__DOCZ_CONFIG__;
  vi.unstubAllEnvs();
});

describe("parseTheme", () => {
  it("accepts a theme pkg/theme has, folded", () => {
    expect(parseTheme("paper")).toBe("paper");
    expect(parseTheme(" Classic ")).toBe("classic");
  });

  it("refuses anything else", () => {
    expect(parseTheme("neon")).toBeUndefined();
    expect(parseTheme('paper"><script>')).toBeUndefined();
    expect(parseTheme(7)).toBeUndefined();
    expect(parseTheme(undefined)).toBeUndefined();
  });
});

describe("defaultTheme", () => {
  it("is paper with nothing configured", () => {
    expect(defaultTheme()).toBe("paper");
  });

  it("takes the injected theme, then VITE_THEME", () => {
    vi.stubEnv("VITE_THEME", "paper");
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    expect(defaultTheme()).toBe("classic");
    window.__DOCZ_CONFIG__ = {};
    vi.stubEnv("VITE_THEME", "classic");
    expect(defaultTheme()).toBe("classic");
  });

  it("lets an invalid injected value fall through", () => {
    window.__DOCZ_CONFIG__ = { theme: "neon" };
    expect(defaultTheme()).toBe("paper");
  });
});

describe("storedTheme", () => {
  it("prefers the reader's valid pick over the default", () => {
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    localStorage.setItem("docz:theme", "paper");
    expect(storedTheme()).toBe("paper");
  });

  it("ignores a stored value it does not know", () => {
    window.__DOCZ_CONFIG__ = { theme: "classic" };
    localStorage.setItem("docz:theme", "neon");
    expect(storedTheme()).toBe("classic");
  });
});
