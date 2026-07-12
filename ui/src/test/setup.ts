import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";

import { server } from "@/test/server";

// jsdom has no layout, so it implements neither of these even though
// the DOM types claim they exist; cmdk calls both. Assigned outright —
// probing them first trips the type-aware lint rules.
Element.prototype.scrollIntoView = () => {
  /* layout no-op */
};
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
});

afterAll(() => {
  server.close();
});
