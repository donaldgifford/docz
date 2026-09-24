import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MermaidBlock, _resetMermaidBlock } from "@/markdown/mermaid-block";

/*
 * mermaid can't actually render in jsdom (no SVG measurement), so the
 * module is mocked with a controllable render — the REAL render path,
 * strict-mode encoding included, is verified in e2e against the
 * preview build.
 */
const mermaidMock = vi.hoisted(() => ({
  initialize: vi.fn(),
  render: vi.fn(),
}));
vi.mock("mermaid", () => ({ default: mermaidMock }));

const SOURCE = "flowchart TD\n  A --> B";

beforeEach(() => {
  _resetMermaidBlock();
  mermaidMock.render.mockReset();
});

describe("MermaidBlock", () => {
  it("renders the diagram as an img-role figure with a caption", async () => {
    mermaidMock.render.mockResolvedValue({ svg: "<svg><text>ok</text></svg>" });
    const { container } = render(
      <MermaidBlock source={SOURCE} caption="fig 1 · order flow" />,
    );

    const img = await screen.findByRole("img", { name: "fig 1 · order flow" });
    await waitFor(() => {
      expect(img.querySelector("svg")).not.toBeNull();
    });
    expect(
      container.querySelector("figure.mermaid-figure figcaption")?.textContent,
    ).toBe("fig 1 · order flow");
  });

  it("labels the figure from the first source line without a caption", async () => {
    mermaidMock.render.mockResolvedValue({ svg: "<svg></svg>" });
    render(<MermaidBlock source={SOURCE} />);

    expect(
      await screen.findByRole("img", { name: "mermaid diagram: flowchart TD" }),
    ).toBeInTheDocument();
  });

  it("serves repeat mounts from the SVG cache", async () => {
    mermaidMock.render.mockResolvedValue({ svg: "<svg></svg>" });
    const first = render(<MermaidBlock source={SOURCE} />);
    await screen.findByRole("img");
    first.unmount();

    render(<MermaidBlock source={SOURCE} />);
    expect(await screen.findByRole("img")).toBeInTheDocument();
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
  });

  it("keeps the source visible when rendering fails", async () => {
    mermaidMock.render.mockRejectedValue(new Error("parse error"));
    const { container } = render(<MermaidBlock source={SOURCE} />);

    await waitFor(() => {
      expect(
        container.querySelector('[data-mermaid-fallback="failed"]'),
      ).not.toBeNull();
    });
    const fallback = screen.getByRole("region", {
      name: "mermaid diagram source",
    });
    expect(fallback.textContent).toContain("flowchart TD");
    expect(container.querySelector("svg")).toBeNull();
  });
});
