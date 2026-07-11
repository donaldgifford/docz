import { useEffect, useRef, useState } from "react";

import { typeColor } from "@/lib/colors";

/*
 * Directory filter controls per the mockup's controls-row + chips.
 * Both are dumb: they render facet counts handed to them and report
 * picks upward — the route owns the URL state (DESIGN-0001: the URL is
 * the only source of filter truth).
 */

function PickerItem({
  label,
  count,
  selected,
  onClick,
}: {
  label: string;
  count: number;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex w-full items-center justify-between gap-6 px-3 py-[0.4rem] text-left font-mono text-[12px] hover:bg-bg-raised ${
        selected ? "text-fg-primary" : "text-fg-tertiary"
      }`}
    >
      <span>{label}</span> <span className="text-fg-muted">{count}</span>
    </button>
  );
}

/** Dropdown scoping the directory to one repo, with per-repo counts. */
export function RepoPicker({
  current,
  counts,
  onPick,
}: {
  current: string | null;
  counts: Record<string, number>;
  onPick: (repo: string | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    function closeOnOutsidePress(event: PointerEvent) {
      if (
        event.target instanceof Node &&
        rootRef.current?.contains(event.target) === true
      ) {
        return;
      }
      setOpen(false);
    }
    document.addEventListener("pointerdown", closeOnOutsidePress);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePress);
    };
  }, [open]);

  const repos = Object.entries(counts).sort(([a], [b]) => a.localeCompare(b));
  const total = repos.reduce((sum, [, count]) => sum + count, 0);

  const pick = (repo: string | null) => {
    setOpen(false);
    onPick(repo);
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-haspopup="true"
        aria-expanded={open}
        onClick={() => {
          setOpen((value) => !value);
        }}
        className="flex items-center gap-2 border border-border-default bg-bg-raised px-3 py-[0.35rem] font-mono text-[12px] text-fg-secondary hover:border-border-strong"
      >
        <span className="text-fg-muted">repo:</span>{" "}
        <span>{current ?? "all"}</span>
        <span aria-hidden className="text-[9px] text-fg-muted">
          ▾
        </span>
      </button>
      {open && (
        <div className="absolute top-full left-0 z-10 mt-1 min-w-60 border border-border-strong bg-bg-elevated py-1 shadow-[0_10px_36px_rgba(0,0,0,0.55)]">
          <PickerItem
            label="all repos"
            count={total}
            selected={current === null}
            onClick={() => {
              pick(null);
            }}
          />
          <div className="my-1 border-t border-border-hairline" />
          {repos.map(([repo, count]) => (
            <PickerItem
              key={repo}
              label={repo}
              count={count}
              selected={current === repo}
              onClick={() => {
                pick(repo);
              }}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function Chip({
  label,
  dotColor,
  pressed,
  onClick,
}: {
  label: string;
  dotColor?: string;
  pressed: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={`inline-flex items-center gap-[0.4rem] border px-[0.7rem] py-[0.25rem] font-mono text-[11.5px] tracking-[0.04em] uppercase ${
        pressed
          ? "border-border-strong bg-bg-raised text-fg-primary"
          : "border-border-default text-fg-tertiary hover:text-fg-secondary"
      }`}
    >
      {dotColor !== undefined && (
        <span
          aria-hidden
          className="size-[7px]"
          style={{ background: dotColor }}
        />
      )}
      {label}
    </button>
  );
}

/**
 * Type chips from the union of the `type` facet values — never a
 * hardcoded list. Single-select: searchDocs takes one `type` per query
 * (see src/lib/searchParams.ts), so a chip toggles itself and replaces
 * any other selection.
 */
export function TypeChips({
  available,
  selected,
  onSelect,
}: {
  available: string[];
  selected: string | null;
  onSelect: (type: string | null) => void;
}) {
  if (available.length === 0) {
    return null;
  }
  return (
    <div className="mt-[0.9rem] flex flex-wrap gap-2">
      <Chip
        label="all types"
        pressed={selected === null}
        onClick={() => {
          onSelect(null);
        }}
      />
      {available.map((type) => (
        <Chip
          key={type}
          label={type}
          dotColor={typeColor(type)}
          pressed={selected === type}
          onClick={() => {
            onSelect(selected === type ? null : type);
          }}
        />
      ))}
    </div>
  );
}
