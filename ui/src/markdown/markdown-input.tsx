import type { ComponentPropsWithoutRef } from "react";

/*
 * Input mapping for rendered markdown. The only <input> the sanitizer
 * lets through is the GFM task-list checkbox (type=checkbox, disabled,
 * optionally checked), and it arrives with no accessible name — axe
 * flags that as a critical "label" violation, which the rendering
 * specimen surfaced for every IMPL document. The name states the
 * task's state; the li text that follows is the task itself, so a
 * screen reader hears "Done, checkbox, Ship the release". The schema
 * strips aria-* from inputs, so the name is always ours — it is set
 * after the spread to keep that deterministic.
 */
export function MarkdownInput(props: ComponentPropsWithoutRef<"input">) {
  if (props.type !== "checkbox") {
    return <input {...props} />;
  }
  return (
    <input
      {...props}
      aria-label={props.checked === true ? "Done" : "Not done"}
    />
  );
}
