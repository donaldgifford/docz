import { lazy, Suspense, useEffect, useState } from "react";

/*
 * The palette's eager half: the shortcut that opens it and the import
 * that loads it. The palette itself (command-palette.tsx: cmdk, the
 * design system's dialog and command parts, the search queries) is a
 * chunk of its own, fetched after the first paint, so none of it counts
 * against the eager budget (scripts/bundle-budget.ts). What stays eager
 * is this file: a key listener and a dynamic import.
 */

/** How long after the shell mounts the palette's chunk is fetched. */
const PRELOAD_AFTER_MS = 1500;

const load = () => import("@/components/command-palette");

const CommandPalette = lazy(async () => ({
  default: (await load()).CommandPalette,
}));

/**
 * Starts fetching the palette's chunk. The shell calls it shortly after
 * it mounts, and the search button on hover and focus, so opening the
 * palette usually finds the chunk already there.
 */
export function preloadCommandPalette(): void {
  void load();
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement ||
    target.isContentEditable
  );
}

/**
 * Global shortcuts: ⌘K/Ctrl+K toggles anywhere; "/" opens unless typing
 * in an editable element. Esc is the dialog's own.
 */
function usePaletteShortcuts(
  open: boolean,
  onOpenChange: (open: boolean) => void,
): void {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key === "k") {
        event.preventDefault();
        onOpenChange(!open);
        return;
      }
      if (!open && event.key === "/" && !isEditableTarget(event.target)) {
        event.preventDefault();
        onOpenChange(true);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onOpenChange]);
}

/**
 * The palette, mounted from its first opening on. It stays mounted
 * after that, closed, so a reopened palette still holds its query.
 */
export function LazyCommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  usePaletteShortcuts(open, onOpenChange);

  // Fetch the chunk once the page has settled, so the first ⌘K opens at
  // once and the keys typed straight after it land in the input.
  useEffect(() => {
    const timer = setTimeout(preloadCommandPalette, PRELOAD_AFTER_MS);
    return () => {
      clearTimeout(timer);
    };
  }, []);

  // Adjust-during-render: the first open is what mounts the chunk.
  const [wanted, setWanted] = useState(open);
  if (open && !wanted) {
    setWanted(true);
  }
  if (!wanted) {
    return null;
  }
  return (
    <Suspense fallback={null}>
      <CommandPalette open={open} onOpenChange={onOpenChange} />
    </Suspense>
  );
}
