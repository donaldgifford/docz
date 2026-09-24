import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

async function renderDoc(md: string) {
  const { content } = await renderMarkdown(md);
  return render(<>{content}</>);
}

describe("heading anchor copy", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("copies the absolute section URL and announces it", async () => {
    const writeText = vi.fn<(text: string) => Promise<void>>(() =>
      Promise.resolve(),
    );
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    await renderDoc("## First Section\n\nBody.");
    fireEvent.click(
      screen.getByRole("button", { name: "Copy link to section" }),
    );

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(
        expect.stringMatching(/#first-section$/),
      );
    });
    // Absolute URL, not a bare fragment.
    expect(writeText.mock.calls[0]?.[0]).toMatch(/^http/);
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("link copied");
    });
  });

  it("puts the affordance on the ToC-collected set (h2–h4) only", async () => {
    await renderDoc("# T\n\n## A\n\n### B\n\n#### C\n\n##### D");

    expect(
      screen.getAllByRole("button", { name: "Copy link to section" }),
    ).toHaveLength(3);
  });
});
