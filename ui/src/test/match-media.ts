/*
 * window.matchMedia for jsdom, which has none (DESIGN-0028 §13). The
 * design system's sidebar reads it, and appearance.ts follows the
 * system's colour scheme through it. Every query answers false except
 * the dark-scheme one, which a test moves with setSystemDark().
 */

const DARK_QUERY = "(prefers-color-scheme: dark)";

type ChangeListener =
  | EventListenerOrEventListenerObject
  | ((this: MediaQueryList, event: MediaQueryListEvent) => unknown);

let systemDark = false;
const listeners = new Map<string, Set<ChangeListener>>();

function listenersFor(query: string): Set<ChangeListener> {
  let set = listeners.get(query);
  if (set === undefined) {
    set = new Set();
    listeners.set(query, set);
  }
  return set;
}

function mediaQueryList(query: string): MediaQueryList {
  const set = listenersFor(query);
  const add = (listener: ChangeListener | null) => {
    if (listener !== null) {
      set.add(listener);
    }
  };
  const remove = (listener: ChangeListener | null) => {
    if (listener !== null) {
      set.delete(listener);
    }
  };
  return {
    get matches() {
      return query === DARK_QUERY && systemDark;
    },
    media: query,
    onchange: null,
    addEventListener: (_type: string, listener: ChangeListener | null) => {
      add(listener);
    },
    removeEventListener: (_type: string, listener: ChangeListener | null) => {
      remove(listener);
    },
    addListener: add,
    removeListener: remove,
    dispatchEvent: () => true,
  };
}

/** Installs the stub on window. setup.ts calls it once per test file. */
export function installMatchMedia(): void {
  window.matchMedia = mediaQueryList;
}

/**
 * setSystemDark moves the system's colour scheme and tells every
 * listener on the dark-scheme query, as a real change would.
 */
export function setSystemDark(dark: boolean): void {
  if (systemDark === dark) {
    return;
  }
  systemDark = dark;
  const list = mediaQueryList(DARK_QUERY);
  const event = { matches: dark, media: DARK_QUERY } as MediaQueryListEvent;
  for (const listener of listenersFor(DARK_QUERY)) {
    if (typeof listener === "function") {
      listener.call(list, event);
    } else {
      listener.handleEvent(event);
    }
  }
}

/**
 * resetSystemMode puts the system back on light between tests without
 * announcing it: listeners stay registered, since a module that
 * subscribed once (appearance.ts) is not re-evaluated per test.
 */
export function resetSystemMode(): void {
  systemDark = false;
}
