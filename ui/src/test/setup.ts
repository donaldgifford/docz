import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";

import { installMatchMedia, resetSystemMode } from "@/test/match-media";
import { server } from "@/test/server";

// jsdom has no layout, so it implements neither of these even though
// the DOM types claim they exist; cmdk calls both. Assigned outright —
// probing them first trips the type-aware lint rules.
Element.prototype.scrollIntoView = () => {
  /* layout no-op */
};
// Base UI waits for a closing panel's animations before it unmounts it
// (the sidebar tree's branches, the scroll area in its popover); jsdom
// runs none.
Element.prototype.getAnimations = () => [];
globalThis.ResizeObserver = class {
  observe() {
    /* layout no-op */
  }
  unobserve() {
    /* layout no-op */
  }
  disconnect() {
    /* layout no-op */
  }
};

// jsdom has no matchMedia either. The design system's sidebar reads it
// and appearance.ts follows the system's colour scheme through it
// (DESIGN-0028 §13); a test moves the system with setSystemDark().
installMatchMedia();

beforeAll(() => {
  // Any request MSW doesn't recognize is a test bug, not a passthrough.
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  server.resetHandlers();
  cleanup();
  // jsdom storage persists across tests in a file; a leaked auth
  // return-to stash would arm RestoreAfterLogin in unrelated tests.
  sessionStorage.clear();
  localStorage.clear();
  // The mode is page state too: the system goes back to light, and
  // <html> loses what a test's toggle left on it.
  resetSystemMode();
  document.documentElement.classList.remove("dark");
  document.documentElement.removeAttribute("data-mode");
});

afterAll(() => {
  server.close();
});
