import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { applyStoredMode, useAppearance } from "@/lib/appearance";
import { setSystemDark } from "@/test/match-media";

/*
 * The mode is on <html> twice (DESIGN-0028 §5): data-mode for pkg/theme's
 * document themes, and the dark class for the design system's parts.
 * These hold the two together for a stored light mode, a stored dark
 * mode, and the system followed in both directions, from
 * applyStoredMode() (before first paint) and from the toggle.
 *
 * A file of its own, so applyStoredMode()'s one media-query listener is
 * registered on setup.ts's matchMedia and not on another test's stub.
 */

const root = document.documentElement;

function onHtml(): { dataMode: string | null; dark: boolean } {
  return {
    dataMode: root.getAttribute("data-mode"),
    dark: root.classList.contains("dark"),
  };
}

describe("applyStoredMode", () => {
  it("sets a stored light mode, whatever the system says", () => {
    setSystemDark(true);
    localStorage.setItem("docz:mode", "light");
    applyStoredMode();
    expect(onHtml()).toEqual({ dataMode: "light", dark: false });
  });

  it("sets a stored dark mode, whatever the system says", () => {
    setSystemDark(false);
    localStorage.setItem("docz:mode", "dark");
    applyStoredMode();
    expect(onHtml()).toEqual({ dataMode: "dark", dark: true });
  });

  it("follows a dark system with nothing stored", () => {
    setSystemDark(true);
    applyStoredMode();
    expect(onHtml()).toEqual({ dataMode: null, dark: true });
  });

  it("follows a light system with nothing stored", () => {
    setSystemDark(false);
    root.classList.add("dark");
    applyStoredMode();
    expect(onHtml()).toEqual({ dataMode: null, dark: false });
  });

  it("moves the class when the system changes, in both directions", () => {
    applyStoredMode();
    expect(onHtml()).toEqual({ dataMode: null, dark: false });
    setSystemDark(true);
    expect(onHtml()).toEqual({ dataMode: null, dark: true });
    setSystemDark(false);
    expect(onHtml()).toEqual({ dataMode: null, dark: false });
  });

  it("leaves a stored mode alone when the system changes", () => {
    localStorage.setItem("docz:mode", "light");
    applyStoredMode();
    setSystemDark(true);
    expect(onHtml()).toEqual({ dataMode: "light", dark: false });
  });
});

describe("useAppearance", () => {
  it("follows the system in both directions until a mode is picked", () => {
    const { result } = renderHook(() => useAppearance());
    expect(result.current.mode).toBe("light");
    expect(onHtml()).toEqual({ dataMode: null, dark: false });

    act(() => {
      setSystemDark(true);
    });
    expect(result.current.mode).toBe("dark");
    expect(onHtml()).toEqual({ dataMode: null, dark: true });

    act(() => {
      setSystemDark(false);
    });
    expect(result.current.mode).toBe("light");
    expect(onHtml()).toEqual({ dataMode: null, dark: false });
  });

  it("toggles both together, and back", () => {
    const { result } = renderHook(() => useAppearance());
    act(() => {
      result.current.toggleMode();
    });
    expect(onHtml()).toEqual({ dataMode: "dark", dark: true });
    expect(localStorage.getItem("docz:mode")).toBe("dark");

    act(() => {
      result.current.toggleMode();
    });
    expect(onHtml()).toEqual({ dataMode: "light", dark: false });
    expect(localStorage.getItem("docz:mode")).toBe("light");
  });

  it("toggles away from a dark system to a stored light mode", () => {
    setSystemDark(true);
    const { result } = renderHook(() => useAppearance());
    expect(onHtml()).toEqual({ dataMode: null, dark: true });
    act(() => {
      result.current.toggleMode();
    });
    expect(onHtml()).toEqual({ dataMode: "light", dark: false });
  });

  it("starts from a stored mode", () => {
    setSystemDark(true);
    localStorage.setItem("docz:mode", "light");
    renderHook(() => useAppearance());
    expect(onHtml()).toEqual({ dataMode: "light", dark: false });
  });
});
