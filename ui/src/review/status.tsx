import { useId } from "react";

/*
 * The status select (DESIGN-0024 section 5): the type's statuses with
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
  const id = useId();
  // A document whose status is not one the type allows still shows it.
  const options = statuses.includes(current)
    ? statuses
    : [current, ...statuses];
  return (
    <div className="flex items-center gap-2">
      <label htmlFor={id} className="font-mono text-[12px] text-fg-tertiary">
        Status
      </label>
      <select
        id={id}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        className="border border-border-default bg-bg-base px-2 py-[2px] font-mono text-[12.5px] text-fg-primary"
      >
        {options.map((s) => (
          <option key={s} value={s}>
            {s === current ? `${s} (current)` : s}
          </option>
        ))}
      </select>
    </div>
  );
}
