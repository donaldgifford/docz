import { useEffect, useState, type RefObject } from "react";

import {
  anchorFromRange,
  rangeForAnchor,
  type SelectionAnchor,
} from "@/review/anchor";
import { ReviewButton } from "@/review/controls";
import { inDocumentOrder, type AnnotationDraft } from "@/review/draft";

import type { Heading } from "@/cli/__generated__/inspect";

/*
 * Annotations (DESIGN-0024 section 5). Selecting text in the rendered
 * document opens a popover with four actions: comment, replace, delete,
 * and looks good. A selection spanning blocks may carry a comment only:
 * replace and delete are refused rather than truncated. A document-level
 * comment box sits above, the sidebar lists every annotation in
 * document order for editing or removal, and existing annotations are
 * painted with the CSS Custom Highlight API, so no DOM is inserted into
 * the sanitized markdown.
 */

/** The name of the highlight docz-review paints; styled in tokens.css. */
export const HIGHLIGHT = "docz-review";

export interface PendingSelection {
  anchor: SelectionAnchor;
  /** Where to put the popover, in viewport coordinates. */
  rect: { top: number; left: number; bottom: number };
}

/**
 * The current selection inside `root` as an anchor, or null. A selection
 * starting inside `[data-no-annotate]` (the diff) is ignored, and so is
 * a mouse-up inside the popover itself, so clicking its buttons keeps it.
 */
export function useSelectionAnchor(
  root: RefObject<HTMLElement | null>,
  headings: readonly Heading[],
): [PendingSelection | null, () => void] {
  const [pending, setPending] = useState<PendingSelection | null>(null);

  useEffect(() => {
    const read = (event: Event) => {
      if (
        event.target instanceof Element &&
        event.target.closest("[data-review-popover]") !== null
      ) {
        return;
      }
      const selection = document.getSelection();
      const container = root.current;
      if (
        selection === null ||
        container === null ||
        selection.isCollapsed ||
        selection.rangeCount === 0
      ) {
        setPending(null);
        return;
      }
      const range = selection.getRangeAt(0);
      const start =
        range.startContainer instanceof Element
          ? range.startContainer
          : range.startContainer.parentElement;
      if (
        !container.contains(range.startContainer) ||
        start?.closest("[data-no-annotate]") != null
      ) {
        setPending(null);
        return;
      }
      const anchor = anchorFromRange(range, headings);
      // jsdom has no layout, so no Range rect; a browser always does.
      const box =
        typeof range.getBoundingClientRect === "function"
          ? range.getBoundingClientRect()
          : { top: 0, left: 0, bottom: 0 };
      setPending(
        anchor === null || anchor.exact === ""
          ? null
          : {
              anchor,
              rect: { top: box.top, left: box.left, bottom: box.bottom },
            },
      );
    };
    document.addEventListener("mouseup", read);
    document.addEventListener("keyup", read);
    return () => {
      document.removeEventListener("mouseup", read);
      document.removeEventListener("keyup", read);
    };
  }, [root, headings]);

  return [
    pending,
    () => {
      setPending(null);
    },
  ];
}

const newId = () => crypto.randomUUID();

export function SelectionPopover({
  pending,
  onAdd,
  onCancel,
}: {
  pending: PendingSelection;
  onAdd: (annotation: AnnotationDraft) => void;
  onCancel: () => void;
}) {
  const [mode, setMode] = useState<"comment" | "replace" | null>(null);
  const [text, setText] = useState("");
  const { anchor } = pending;
  const add = (kind: AnnotationDraft["kind"], extra = {}) => {
    onAdd({ id: newId(), kind, anchor, ...extra });
  };

  return (
    <div
      data-review-popover
      role="dialog"
      aria-label="Annotate selection"
      style={{ top: pending.rect.bottom + 6, left: pending.rect.left }}
      className="fixed z-50 w-[320px] border border-border-strong bg-bg-elevated p-2 shadow-lg"
    >
      <p className="mb-2 line-clamp-2 font-mono text-[12px] text-fg-tertiary">
        “{anchor.selection}”
      </p>
      {mode === null ? (
        <div className="flex flex-wrap gap-1">
          <ReviewButton
            onClick={() => {
              setMode("comment");
            }}
          >
            Comment
          </ReviewButton>
          <ReviewButton
            disabled={anchor.multiBlock}
            title={
              anchor.multiBlock ? "Only a comment may span blocks" : undefined
            }
            onClick={() => {
              setMode("replace");
            }}
          >
            Replace
          </ReviewButton>
          <ReviewButton
            disabled={anchor.multiBlock}
            title={
              anchor.multiBlock ? "Only a comment may span blocks" : undefined
            }
            onClick={() => {
              add("delete");
            }}
          >
            Delete
          </ReviewButton>
          <ReviewButton
            onClick={() => {
              add("looks-good");
            }}
          >
            Looks good
          </ReviewButton>
          <ReviewButton onClick={onCancel}>Cancel</ReviewButton>
        </div>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (mode === "comment") {
              add("comment", { text });
            } else {
              add("replace", { replacement: text });
            }
          }}
        >
          <label className="block font-mono text-[12px] text-fg-tertiary">
            {mode === "comment" ? "Comment" : "Replace with"}
            <textarea
              className="mt-1 block w-full border border-border-default bg-bg-base p-2 text-fg-primary"
              value={text}
              onChange={(e) => {
                setText(e.target.value);
              }}
            />
          </label>
          <div className="mt-2 flex gap-1">
            <ReviewButton
              type="submit"
              variant="primary"
              disabled={mode === "comment" && text.trim() === ""}
            >
              Add
            </ReviewButton>
            <ReviewButton onClick={onCancel}>Cancel</ReviewButton>
          </div>
        </form>
      )}
    </div>
  );
}

export function DocumentComment({
  onAdd,
}: {
  onAdd: (annotation: AnnotationDraft) => void;
}) {
  const [text, setText] = useState("");
  return (
    <form
      className="flex flex-col gap-1"
      onSubmit={(e) => {
        e.preventDefault();
        onAdd({ id: newId(), kind: "document", text: text.trim() });
        setText("");
      }}
    >
      <label className="font-mono text-[12px] text-fg-tertiary">
        Comment on the whole document
        <textarea
          className="mt-1 block w-full border border-border-default bg-bg-base p-2 text-fg-primary"
          value={text}
          onChange={(e) => {
            setText(e.target.value);
          }}
        />
      </label>
      <ReviewButton type="submit" disabled={text.trim() === ""}>
        Add document comment
      </ReviewButton>
    </form>
  );
}

const KIND_LABEL: Record<AnnotationDraft["kind"], string> = {
  comment: "comment",
  replace: "replace",
  delete: "delete",
  document: "document",
  "looks-good": "looks good",
};

function AnnotationItem({
  annotation,
  onEdit,
  onRemove,
}: {
  annotation: AnnotationDraft;
  onEdit: (patch: Partial<AnnotationDraft>) => void;
  onRemove: () => void;
}) {
  const field =
    annotation.kind === "replace"
      ? "replacement"
      : annotation.kind === "comment" || annotation.kind === "document"
        ? "text"
        : null;
  return (
    <li
      data-annotation={annotation.id}
      className="border border-border-hairline bg-bg-raised p-2"
    >
      <p className="font-mono text-[11.5px] tracking-[0.04em] text-fg-tertiary uppercase">
        {KIND_LABEL[annotation.kind]}
        {annotation.anchor !== undefined &&
          ` · line ${String(annotation.anchor.line)}`}
      </p>
      {annotation.anchor !== undefined && (
        <p className="mt-1 line-clamp-2 text-[13px] text-fg-secondary">
          “{annotation.anchor.selection}”
        </p>
      )}
      {field !== null && (
        <label className="mt-1 block font-mono text-[12px] text-fg-tertiary">
          {field === "replacement" ? "Replace with" : "Text"}
          <textarea
            className="mt-1 block w-full border border-border-default bg-bg-base p-1 text-fg-primary"
            value={annotation[field] ?? ""}
            onChange={(e) => {
              onEdit({ [field]: e.target.value });
            }}
          />
        </label>
      )}
      <ReviewButton className="mt-1" onClick={onRemove}>
        Remove
      </ReviewButton>
    </li>
  );
}

export function AnnotationList({
  annotations,
  onEdit,
  onRemove,
}: {
  annotations: readonly AnnotationDraft[];
  onEdit: (id: string, patch: Partial<AnnotationDraft>) => void;
  onRemove: (id: string) => void;
}) {
  if (annotations.length === 0) {
    return (
      <p className="font-mono text-[12px] text-fg-muted">
        Select text in the document to annotate it.
      </p>
    );
  }
  return (
    <ol aria-label="Annotations" className="flex flex-col gap-2">
      {inDocumentOrder(annotations).map((a) => (
        <AnnotationItem
          key={a.id}
          annotation={a}
          onEdit={(patch) => {
            onEdit(a.id, patch);
          }}
          onRemove={() => {
            onRemove(a.id);
          }}
        />
      ))}
    </ol>
  );
}

/**
 * Paints every anchored annotation found on the page as one named
 * highlight, inserting no DOM. A browser without the API paints nothing.
 * Returns the number of annotations painted.
 */
export function paintAnnotations(
  root: HTMLElement,
  annotations: readonly AnnotationDraft[],
): number {
  if (typeof CSS === "undefined" || !("highlights" in CSS)) {
    return 0;
  }
  const ranges = annotations.flatMap((a) => {
    const range =
      a.anchor === undefined ? null : rangeForAnchor(root, a.anchor);
    return range === null ? [] : [range];
  });
  CSS.highlights.set(HIGHLIGHT, new Highlight(...ranges));
  return ranges.length;
}

/** Keeps the page's highlight in step with the annotations. */
export function useAnnotationPaint(
  root: RefObject<HTMLElement | null>,
  annotations: readonly AnnotationDraft[],
  rendered: unknown,
): void {
  useEffect(() => {
    if (root.current !== null) {
      paintAnnotations(root.current, annotations);
    }
  }, [root, annotations, rendered]);
  useEffect(
    () => () => {
      if (typeof CSS !== "undefined" && "highlights" in CSS) {
        CSS.highlights.delete(HIGHLIGHT);
      }
    },
    [],
  );
}
