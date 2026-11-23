import { Badge } from "@donaldgifford/design-system/components/badge";
import { Button } from "@donaldgifford/design-system/components/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@donaldgifford/design-system/components/dialog";
import { cn } from "@donaldgifford/design-system/lib/utils";

import type { DocumentState } from "@/review/api";
import type { ComponentProps } from "react";

/*
 * The review page's controls, on the design system's parts (DESIGN-0028
 * §10). Kept here rather than in src/components because nothing else in
 * the site posts anything.
 */

type ButtonProps = ComponentProps<typeof Button>;

/**
 * The page's one button: the package's `Button`, outlined unless it is
 * the primary action. `selected` is the chosen one of a set (a radio, a
 * pressed toggle); the role and the ARIA state are the caller's.
 */
export function ReviewButton({
  variant = "plain",
  className,
  ...rest
}: Omit<ButtonProps, "variant"> & {
  variant?: "plain" | "primary" | "selected";
}) {
  return (
    <Button
      variant={variant === "primary" ? "default" : "outline"}
      className={cn(
        // A disabled button keeps its pointer, so its `title` still says
        // why it is disabled.
        "disabled:pointer-events-auto disabled:cursor-not-allowed",
        variant === "selected" && "border-primary bg-muted text-foreground",
        className,
      )}
      {...rest}
    />
  );
}

/** The class of a text box or a one-line input on the page. */
export const FIELD_CLASS =
  "mt-1 block w-full rounded-sm border bg-background px-2 py-1.5 text-base text-foreground";

/** The class of the small label over a field or a control. */
export const LABEL_CLASS = "block text-xs text-muted-foreground";

const STATE_VARIANT = {
  open: "pending",
  changes: "warning",
  approved: "approved",
  dismissed: "ignored",
} as const satisfies Record<
  DocumentState,
  ComponentProps<typeof Badge>["variant"]
>;

/** A document's place in the round, as a badge. */
export function StateBadge({ state }: { state: DocumentState }) {
  return (
    <Badge data-state-badge={state} variant={STATE_VARIANT[state]}>
      {state}
    </Badge>
  );
}

/**
 * A question the page asks before it throws something away. Cancel and
 * Escape close it and do nothing; the confirm button closes it and runs
 * `onConfirm`.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirm,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  /** The confirm button's label. */
  confirm: string;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle className="leading-snug">{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>
            Cancel
          </DialogClose>
          <Button
            onClick={() => {
              onOpenChange(false);
              onConfirm();
            }}
          >
            {confirm}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
