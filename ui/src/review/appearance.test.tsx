import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  applyStoredMode,
  applyTheme,
  hasModes,
  storedMode,
  useAppearance,
} from "@/review/appearance";
import { AppearanceControls } from "@/review/controls";

const root = document.documentElement;

/** Stubs matchMedia with a system mode that a test can flip. */
function stubSystem(dark: boolean) {
  const listeners = new Set<() => void>();
  const query = {
    get matches() {
      return dark;
    },
    addEventListener: (_: string, fn: () => void) => listeners.add(fn),
    removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
  };
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => query),
  );
  return (next: boolean) => {
    dark = next;
    for (const fn of listeners) fn();
  };
}

function Controls({ theme }: { theme: string }) {
  return <AppearanceControls appearance={useAppearance(theme)} />;
}

afterEach(() => {
  vi.unstubAllGlobals();
  root.removeAttribute("data-mode");
  root.setAttribute("data-theme", "paper");
});

describe("stored appearance", () => {
  it("follows the system with nothing stored", () => {
    expect(storedMode()).toBeNull();
    root.setAttribute("data-mode", "dark");
    applyStoredMode();
    expect(root.hasAttribute("data-mode")).toBe(false);
  });

  it("applies the mode an earlier session stored", () => {
    localStorage.setItem("docz:mode", "dark");
    applyStoredMode();
    expect(root.getAttribute("data-mode")).toBe("dark");
  });

  it("ignores a mode it does not know, and never stores a theme", () => {
    localStorage.setItem("docz:mode", "dim");
    expect(storedMode()).toBeNull();
    applyTheme("classic");
    expect(root.getAttribute("data-theme")).toBe("classic");
    expect(localStorage.getItem("docz:theme")).toBeNull();
  });

  it("knows which themes have modes", () => {
    expect(hasModes("paper")).toBe(true);
    expect(hasModes("classic")).toBe(false);
  });
});

describe("useAppearance", () => {
  it("follows the system until the reader picks a mode", () => {
    const flip = stubSystem(false);
    const { result } = renderHook(() => useAppearance("paper"));
    expect(result.current.mode).toBe("light");
    act(() => {
      flip(true);
    });
    expect(result.current.mode).toBe("dark");
    expect(root.hasAttribute("data-mode")).toBe(false);
  });

  it("stores a toggled mode under docz:mode and announces it", () => {
    stubSystem(false);
    const heard: unknown[] = [];
    const listen = (e: Event) => heard.push((e as CustomEvent).detail);
    document.addEventListener("docz:mode", listen);
    const onChange = vi.fn();
    const { result } = renderHook(() => useAppearance("paper", onChange));
    onChange.mockClear();
    act(() => {
      result.current.toggleMode();
    });
    document.removeEventListener("docz:mode", listen);
    expect(localStorage.getItem("docz:mode")).toBe("dark");
    expect(root.getAttribute("data-mode")).toBe("dark");
    expect(heard.at(-1)).toEqual({ mode: "dark", theme: "paper" });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("writes the session's theme to <html>", () => {
    const { rerender } = renderHook(({ theme }) => useAppearance(theme), {
      initialProps: { theme: "classic" },
    });
    expect(root.getAttribute("data-theme")).toBe("classic");
    rerender({ theme: "paper" });
    expect(root.getAttribute("data-theme")).toBe("paper");
  });
});

describe("AppearanceControls", () => {
  it("offers the toggle on paper and no theme selector", async () => {
    stubSystem(false);
    const user = userEvent.setup();
    render(<Controls theme="paper" />);
    await user.click(
      screen.getByRole("button", { name: "Switch to dark mode" }),
    );
    expect(
      screen.getByRole("button", { name: "Switch to light mode" }),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Theme")).toBeNull();
  });

  it("offers nothing on classic, which has one look", () => {
    render(<Controls theme="classic" />);
    expect(screen.queryByRole("button", { name: /Switch to/ })).toBeNull();
  });
});
