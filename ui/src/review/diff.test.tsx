import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  compareRounds,
  DIFF_CHAR_LIMIT,
  markChangedRegions,
  RoundDiff,
  sourceRegions,
} from "@/review/diff";
import { SOURCE } from "@/review/testing";

const edit = (from: string, to: string) => SOURCE.replace(from, to);

describe("sourceRegions", () => {
  it("reads kind, heading, ordinal, and own text from the markers", () => {
    const regions = sourceRegions(SOURCE);
    expect(regions.map((r) => [r.kind, r.heading, r.ordinal])).toEqual([
      ["goals", "Goals", 0],
      ["open-questions", "Open Questions", 0],
    ]);
    expect(regions[0]?.own).toContain("We use **Postgres**");
  });

  it("leaves a child's lines out of its parent's own text", () => {
    const nested = [
      "<!--docz:phase:start-->",
      "### Phase 1: A",
      "<!--docz:tasks:start-->",
      "- [ ] one",
      "<!--docz:tasks:end-->",
      "<!--docz:phase:end-->",
    ].join("\n");
    const [phase, tasks] = sourceRegions(nested);
    expect(phase?.own).toBe("### Phase 1: A");
    expect(tasks?.own).toBe("- [ ] one");
  });

  it("ignores markers inside a code fence", () => {
    expect(sourceRegions("```\n<!--docz:goals:start-->\n```\n")).toEqual([]);
  });
});

describe("compareRounds", () => {
  it("marks only the region whose own text changed", () => {
    const got = compareRounds(SOURCE, edit("**Postgres**", "**SQLite**"));
    expect([...got.changed]).toEqual(["goals:0"]);
    expect(got.deleted).toEqual([]);
    expect(got.lines?.some((c) => c.added)).toBe(true);
  });

  it("does not mark a parent for its child's change", () => {
    const before = [
      "<!--docz:phase:start-->",
      "### Phase 1: A",
      "<!--docz:tasks:start-->",
      "- [ ] one",
      "<!--docz:tasks:end-->",
      "<!--docz:phase:end-->",
    ].join("\n");
    const got = compareRounds(before, before.replace("[ ] one", "[x] one"));
    expect([...got.changed]).toEqual(["tasks:0"]);
  });

  it("lists deleted regions and frontmatter changes", () => {
    const after = edit("status: Draft", "status: In Review").replace(
      /<!--docz:goals:start-->[\s\S]*?<!--docz:goals:end-->\n/,
      "",
    );
    const got = compareRounds(SOURCE, after);
    expect(got.deleted).toEqual([{ kind: "goals", heading: "Goals" }]);
    expect(got.frontmatter).toEqual([
      { key: "status", before: "Draft", after: "In Review" },
    ]);
  });

  it("matches a region by heading, so a new one is changed and the old deleted", () => {
    const got = compareRounds(SOURCE, edit("## Goals", "## Aims"));
    expect([...got.changed]).toEqual(["goals:0"]);
    expect(got.deleted).toEqual([{ kind: "goals", heading: "Goals" }]);
  });

  it("falls back to the marks past the size bound", () => {
    const big = `${SOURCE}\n${"x".repeat(DIFF_CHAR_LIMIT)}`;
    const got = compareRounds(SOURCE, big);
    expect(got.lines).toBeNull();
    expect(got.changed.size).toBe(0);
  });
});

describe("RoundDiff", () => {
  it("collapses unchanged runs and refuses annotation", async () => {
    const user = userEvent.setup();
    const got = compareRounds(SOURCE, edit("**Postgres**", "**SQLite**"));
    const { container } = render(<RoundDiff comparison={got} />);
    const section = screen.getByRole("region", {
      name: "Changes since the previous round",
    });
    expect(section).toHaveAttribute("data-no-annotate");
    expect(container.querySelector('[data-diff="added"]')).toHaveTextContent(
      "+ We use **SQLite** for the store,",
    );
    const more = screen.getAllByRole("button", { name: /unchanged lines/ });
    const before = container.querySelectorAll('[data-diff="same"]').length;
    const first = more[0];
    if (first === undefined) {
      throw new Error("no collapsed run");
    }
    await user.click(first);
    expect(
      container.querySelectorAll('[data-diff="same"]').length,
    ).toBeGreaterThan(before);
  });

  it("says when the change is too large to show", () => {
    const got = compareRounds(
      SOURCE,
      `${SOURCE}${"x".repeat(DIFF_CHAR_LIMIT)}`,
    );
    render(<RoundDiff comparison={got} />);
    expect(
      screen.getByText(/too large to show line by line/),
    ).toBeInTheDocument();
  });
});

describe("markChangedRegions", () => {
  it("sets and clears the mark on region sections", () => {
    document.body.innerHTML =
      '<section data-region="goals" data-ordinal="0"></section>' +
      '<section data-region="open-questions" data-ordinal="0" data-changed="true"></section>';
    markChangedRegions(document.body, new Set(["goals:0"]));
    const [goals, questions] = document.querySelectorAll("section");
    expect(goals).toHaveAttribute("data-changed", "true");
    expect(questions).not.toHaveAttribute("data-changed");
  });
});
