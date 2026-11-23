import { SegmentedControl } from "@donaldgifford/design-system/blocks/segmented-control";

import { LABEL_CLASS } from "@/review/controls";

/*
 * The status control (DESIGN-0024 section 5): the type's statuses with
 * the document's current one chosen. Choosing another is actionable and
 * the feedback says "Status: set to …".
 */
export function StatusSelect({
  statuses,
  current,
  value,
  onChange,
}: {
  statuses: readonly string[];
  /** The status the document has now. */
  current: string;
  value: string;
  onChange: (status: string) => void;
}) {
  // A document whose status is not one the type allows still shows it;
  // a document with no status has none to show.
  const all =
    current === "" || statuses.includes(current)
      ? statuses
      : [current, ...statuses];
  return (
    <div className="flex flex-col gap-1.5">
      <span aria-hidden className={LABEL_CLASS}>
        Status
      </span>
      <SegmentedControl
        label="Status"
        // A type may have more statuses than one row of the sidebar
        // holds, so the segments wrap and each keeps a control's height.
        className="h-auto! max-w-full flex-wrap justify-start [&_[data-slot=tabs-trigger]]:h-(--control-h)"
        options={all.map((s) => ({
          value: s,
          label: s === current ? `${s} (current)` : s,
        }))}
        value={value}
        onValueChange={onChange}
      />
    </div>
  );
}
