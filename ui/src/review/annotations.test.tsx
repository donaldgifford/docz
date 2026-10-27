import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderMarkdown } from "@/markdown/processor";
import {
  AnnotationList,
  DocumentComment,
  HIGHLIGHT,
  paintAnnotations,
  SelectionPopover,
  useSelectionAnchor,
  type PendingSelection,
} from "@/review/annotations";
import { INSPECT, SOURCE } from "@/review/testing";

import type { SelectionAnchor } from "@/review/anchor";
import type { AnnotationDraft } from "@/review/draft";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(() => Promise.reject(new Error("jsdom"))),
  },
}));

const anchor = (over: Partial<SelectionAnchor> = {}): SelectionAnchor => ({
  region: "goals",
  ordinal: 0,
  heading: "Goals",
  exact: "Postgres",
  prefix: "We use ",
  suffix: " for the store,",
  line: 14,
  selection: "Postgres",
  multiBlock: false,
  ...over,
});

const pending = (over: Partial<SelectionAnchor> = {}): PendingSelection => ({
  anchor: anchor(over),
  rect: { top: 0, left: 0, bottom: 0 },
});

describe("SelectionPopover", () => {
  it("adds a comment with its text", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn<(a: AnnotationDraft) => void>();
    render(
      <SelectionPopover pending={pending()} onAdd={onAdd} onCancel={vi.fn()} />,
    );
    await user.click(screen.getByRole("button", { name: "Comment" }));
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
    await user.type(screen.getByLabelText("Comment"), "Why not SQLite?");
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(onAdd.mock.lastCall?.[0]).toMatchObject({
      kind: "comment",
      text: "Why not SQLite?",
      anchor: { exact: "Postgres", line: 14 },
    });
  });

  it("adds a replace, a delete, and a looks good", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn<(a: AnnotationDraft) => void>();
    render(
      <SelectionPopover pending={pending()} onAdd={onAdd} onCancel={vi.fn()} />,
    );
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(screen.getByRole("button", { name: "Looks good" }));
    await user.click(screen.getByRole("button", { name: "Replace" }));
    await user.type(screen.getByLabelText("Replace with"), "SQLite");
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(onAdd.mock.calls.map(([a]) => [a.kind, a.replacement])).toEqual([
      ["delete", undefined],
      ["looks-good", undefined],
      ["replace", "SQLite"],
    ]);
  });

  it("refuses replace and delete on a multi-block selection", () => {
    render(
      <SelectionPopover
        pending={pending({ multiBlock: true })}
        onAdd={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Replace" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Comment" })).toBeEnabled();
  });
});

describe("DocumentComment", () => {
  it("adds a document comment and clears the box", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn<(a: AnnotationDraft) => void>();
    render(<DocumentComment onAdd={onAdd} />);
    const box = screen.getByLabelText("Comment on the whole document");
    await user.type(box, "Shorten the background.");
    await user.click(
      screen.getByRole("button", { name: "Add document comment" }),
    );
    expect(onAdd.mock.lastCall?.[0]).toMatchObject({
      kind: "document",
      text: "Shorten the background.",
    });
    expect(box).toHaveValue("");
  });
});

function nth(items: HTMLElement[], i: number): HTMLElement {
  const item = items[i];
  if (item === undefined) {
    throw new Error(`no item ${String(i)}`);
  }
  return item;
}

describe("AnnotationList", () => {
  const list: AnnotationDraft[] = [
    {
      id: "late",
      kind: "comment",
      anchor: anchor({ line: 30 }),
      text: "later",
    },
    { id: "doc", kind: "document", text: "whole" },
    {
      id: "early",
      kind: "replace",
      anchor: anchor({ line: 14 }),
      replacement: "x",
    },
  ];

  it("lists in document order and edits and removes", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    const onRemove = vi.fn();
    render(
      <AnnotationList annotations={list} onEdit={onEdit} onRemove={onRemove} />,
    );
    const items = within(
      screen.getByRole("list", { name: "Annotations" }),
    ).getAllByRole("listitem");
    expect(items.map((i) => i.getAttribute("data-annotation"))).toEqual([
      "doc",
      "early",
      "late",
    ]);
    await user.type(within(nth(items, 1)).getByLabelText("Replace with"), "y");
    expect(onEdit).toHaveBeenLastCalledWith("early", { replacement: "xy" });
    await user.click(
      within(nth(items, 2)).getByRole("button", { name: "Remove" }),
    );
    expect(onRemove).toHaveBeenCalledWith("late");
  });
});

async function renderPage() {
  const { content } = await renderMarkdown(SOURCE, {
    review: true,
    regions: INSPECT.regions,
  });
  return content;
}

function Page({
  content,
  onPending,
}: {
  content: React.ReactNode;
  onPending: (p: PendingSelection | null) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [current] = useSelectionAnchor(ref, INSPECT.headings);
  onPending(current);
  return (
    <MemoryRouter>
      <div ref={ref}>{content}</div>
      <p data-no-annotate>diff text</p>
    </MemoryRouter>
  );
}

function selectText(node: Node, from: number, to: number) {
  const range = document.createRange();
  range.setStart(node, from);
  range.setEnd(node, to);
  const selection = document.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
  act(() => {
    document.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
  });
}

describe("useSelectionAnchor", () => {
  it("turns a selection in the document into an anchor", async () => {
    const content = await renderPage();
    const seen = vi.fn<(p: PendingSelection | null) => void>();
    render(<Page content={content} onPending={seen} />);
    const strong = document.querySelector("strong");
    selectText(strong?.firstChild as Node, 0, 8);
    expect(seen.mock.lastCall?.[0]?.anchor).toMatchObject({
      exact: "Postgres",
      region: "goals",
      heading: "Goals",
      line: 14,
    });
  });

  it("ignores a selection in diff text", async () => {
    const content = await renderPage();
    const seen = vi.fn<(p: PendingSelection | null) => void>();
    render(<Page content={content} onPending={seen} />);
    const diff = screen.getByText("diff text");
    selectText(diff.firstChild as Node, 0, 4);
    expect(seen.mock.lastCall?.[0]).toBeNull();
  });
});

describe("paintAnnotations", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("paints each anchor found on the page as one highlight", async () => {
    const highlights = new Map<string, { ranges: Range[] }>();
    vi.stubGlobal("CSS", { highlights });
    vi.stubGlobal(
      "Highlight",
      class {
        ranges: Range[];
        constructor(...ranges: Range[]) {
          this.ranges = ranges;
        }
      },
    );
    const content = await renderPage();
    const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
    const painted = paintAnnotations(container, [
      { id: "1", kind: "comment", anchor: anchor(), text: "x" },
      {
        id: "2",
        kind: "looks-good",
        anchor: anchor({ exact: "Valkey for", line: 15 }),
      },
      {
        id: "3",
        kind: "comment",
        anchor: anchor({ exact: "not on the page" }),
      },
      { id: "4", kind: "document", text: "no anchor" },
    ]);
    expect(painted).toBe(2);
    expect(highlights.get(HIGHLIGHT)?.ranges.map((r) => r.toString())).toEqual([
      "Postgres",
      "Valkey for",
    ]);
  });

  it("paints nothing without the API", async () => {
    vi.stubGlobal("CSS", {});
    const content = await renderPage();
    const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
    expect(
      paintAnnotations(container, [
        { id: "1", kind: "comment", anchor: anchor(), text: "x" },
      ]),
    ).toBe(0);
  });
});
