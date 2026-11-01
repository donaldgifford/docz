import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppearanceControls } from "@/components/appearance-controls";
import { applyStoredMode, storedMode, useAppearance } from "@/lib/appearance";

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

function Controls() {
  const { mode, toggleMode } = useAppearance();
  return <AppearanceControls mode={mode} onToggleMode={toggleMode} />;
}

afterEach(() => {
  vi.unstubAllGlobals();
  root.removeAttribute("data-mode");
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

  it("ignores a mode it does not know", () => {
    localStorage.setItem("docz:mode", "dim");
    expect(storedMode()).toBeNull();
  });
});

describe("useAppearance", () => {
  it("follows the system until the reader picks a mode", () => {
    const flip = stubSystem(false);
    const { result } = renderHook(() => useAppearance());
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
    const { result } = renderHook(() => useAppearance(onChange));
    onChange.mockClear();
    act(() => {
      result.current.toggleMode();
    });
    document.removeEventListener("docz:mode", listen);
    expect(localStorage.getItem("docz:mode")).toBe("dark");
    expect(root.getAttribute("data-mode")).toBe("dark");
    expect(heard.at(-1)).toEqual({ mode: "dark" });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("never puts a theme on <html>", () => {
    renderHook(() => useAppearance());
    expect(root.hasAttribute("data-theme")).toBe(false);
  });
});

describe("AppearanceControls", () => {
  it("offers the toggle and no theme selector", async () => {
    stubSystem(false);
    const user = userEvent.setup();
    render(<Controls />);
    await user.click(
      screen.getByRole("button", { name: "Switch to dark mode" }),
    );
    expect(
      screen.getByRole("button", { name: "Switch to light mode" }),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Theme")).toBeNull();
  });
});
