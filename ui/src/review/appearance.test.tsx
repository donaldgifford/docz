import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  applyStoredAppearance,
  storedMode,
  storedTheme,
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

function Controls() {
  return <AppearanceControls appearance={useAppearance()} />;
}

afterEach(() => {
  vi.unstubAllGlobals();
  root.removeAttribute("data-mode");
  root.setAttribute("data-theme", "paper");
});

describe("stored appearance", () => {
  it("defaults to paper following the system", () => {
    expect(storedTheme()).toBe("paper");
    expect(storedMode()).toBeNull();
    root.setAttribute("data-mode", "dark");
    applyStoredAppearance();
    expect(root.getAttribute("data-theme")).toBe("paper");
    expect(root.hasAttribute("data-mode")).toBe(false);
  });

  it("applies what an earlier session stored", () => {
    localStorage.setItem("docz:theme", "classic");
    localStorage.setItem("docz:mode", "dark");
    applyStoredAppearance();
    expect(root.getAttribute("data-theme")).toBe("classic");
    expect(root.getAttribute("data-mode")).toBe("dark");
  });

  it("ignores a value it does not know", () => {
    localStorage.setItem("docz:theme", "neon");
    localStorage.setItem("docz:mode", "dim");
    expect(storedTheme()).toBe("paper");
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
    expect(heard.at(-1)).toEqual({ mode: "dark", theme: "paper" });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("stores the theme under docz:theme", () => {
    const { result } = renderHook(() => useAppearance());
    act(() => {
      result.current.setTheme("classic");
    });
    expect(localStorage.getItem("docz:theme")).toBe("classic");
    expect(root.getAttribute("data-theme")).toBe("classic");
  });
});

describe("AppearanceControls", () => {
  it("offers the toggle on paper and none on classic", async () => {
    stubSystem(false);
    const user = userEvent.setup();
    render(<Controls />);
    await user.click(
      screen.getByRole("button", { name: "Switch to dark mode" }),
    );
    expect(
      screen.getByRole("button", { name: "Switch to light mode" }),
    ).toBeTruthy();
    await user.selectOptions(screen.getByLabelText("Theme"), "classic");
    expect(screen.queryByRole("button", { name: /Switch to/ })).toBeNull();
  });
});
