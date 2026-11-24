import { FilterChip } from "@donaldgifford/design-system/blocks/filter-chip";
import {
  OptionItem,
  OptionSeparator,
} from "@donaldgifford/design-system/blocks/option-rows";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@donaldgifford/design-system/components/popover";
import { useState } from "react";

import { TypeDot } from "@/components/badges";

/*
 * The directory's filters (DESIGN-0028 §7), as the chips of the design
 * system's FilterBar, which the route draws around them. Both are
 * dumb: they render facet counts handed to them and report picks
 * upward — the route owns the URL state (DESIGN-0001: the URL is the
 * only source of filter truth).
 */

/**
 * The `repo = …` chip, which opens the repositories as option rows with
 * each one's count. Picking a row scopes the directory to it and closes
 * the popover.
 */
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

  const repos = Object.entries(counts).sort(([a], [b]) => a.localeCompare(b));
  const total = repos.reduce((sum, [, count]) => sum + count, 0);

  const pick = (repo: string | null) => {
    setOpen(false);
    onPick(repo);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={<FilterChip name="repo" value={current ?? "all"} />}
      />
      <PopoverContent
        align="start"
        aria-label="Repository"
        className="w-auto min-w-60 gap-0 p-1"
      >
        <OptionItem
          current={current === null}
          detail={String(total)}
          onClick={() => {
            pick(null);
          }}
        >
          all repos
        </OptionItem>
        <OptionSeparator />
        {repos.map(([repo, count]) => (
          <OptionItem
            key={repo}
            current={current === repo}
            detail={String(count)}
            onClick={() => {
              pick(repo);
            }}
          >
            {repo}
          </OptionItem>
        ))}
      </PopoverContent>
    </Popover>
  );
}

/**
 * Type chips from the union of the `type` facet values — never a
 * hardcoded list. Single-select: searchDocs takes one `type` per query
 * (see src/lib/searchParams.ts), so one click on a chip turns it on in
 * place of any other, and a click on the one that is on turns it off.
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
    <>
      <FilterChip
        pressed={selected === null}
        onPressedChange={() => {
          onSelect(null);
        }}
      >
        all types
      </FilterChip>
      {available.map((type) => (
        <FilterChip
          key={type}
          pressed={selected === type}
          onPressedChange={(pressed) => {
            onSelect(pressed ? type : null);
          }}
        >
          <TypeDot type={type} />
          {type}
        </FilterChip>
      ))}
    </>
  );
}
