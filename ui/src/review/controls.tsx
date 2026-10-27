import type { ComponentPropsWithoutRef } from "react";

/*
 * The review page's one button, in docz-site's chrome: mono, sharp
 * corners, the accent for the primary action. Kept here rather than in
 * src/components because nothing else in the site posts anything.
 */
export function ReviewButton({
  variant = "plain",
  className = "",
  ...rest
}: ComponentPropsWithoutRef<"button"> & {
  variant?: "plain" | "primary" | "selected";
}) {
  const look = {
    plain: "border-border-default text-fg-secondary hover:bg-bg-hover",
    primary: "border-accent-border bg-accent-bg text-accent hover:bg-bg-hover",
    selected: "border-accent-border bg-bg-active text-fg-primary",
  }[variant];
  return (
    <button
      type="button"
      className={`border px-[10px] py-[3px] font-mono text-[12.5px] disabled:cursor-not-allowed disabled:opacity-50 ${look} ${className}`}
      {...rest}
    />
  );
}
