import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

describe("task-list checkboxes", () => {
  it("get an accessible name from their state and stay inert", async () => {
    const { content } = await renderMarkdown(
      "- [x] Ship the release\n- [ ] Write the release note\n",
    );
    render(<MemoryRouter>{content}</MemoryRouter>);

    const done = screen.getByRole("checkbox", { name: "Done" });
    expect(done).toBeChecked();
    expect(done).toBeDisabled();

    const open = screen.getByRole("checkbox", { name: "Not done" });
    expect(open).not.toBeChecked();
    expect(open).toBeDisabled();
  });

  it("names a raw-HTML checkbox the same way and forces it inert", async () => {
    // The schema keeps only type/disabled on input (disabled is
    // required, so the sanitizer adds it) and strips aria-*.
    const { content } = await renderMarkdown(
      '- <input type="checkbox" checked aria-label="Reviewed"> Design\n',
    );
    render(<MemoryRouter>{content}</MemoryRouter>);

    const box = screen.getByRole("checkbox", { name: "Done" });
    expect(box).toBeChecked();
    expect(box).toBeDisabled();
    expect(screen.queryByRole("checkbox", { name: "Reviewed" })).toBeNull();
  });
});
