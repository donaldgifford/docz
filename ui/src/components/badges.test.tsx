import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  StatusBadge,
  StatusPill,
  TypeBadge,
  TypeDot,
} from "@/components/badges";

/*
 * What docz decides is which variant a status takes and which tone a
 * type takes. How a variant or a tone is drawn is the design system's,
 * and its own tests hold that, so these assert the two attributes the
 * package stamps: data-variant on a Badge, data-tone on a Dot or a
 * toned Badge, and the --tone a custom tone sets inline.
 */

function dot(container: HTMLElement): HTMLElement {
  const found = container.querySelector<HTMLElement>('[data-slot="dot"]');
  if (found === null) {
    throw new Error("no Dot rendered");
  }
  return found;
}

describe("StatusPill", () => {
  it.each([
    ["Draft", "warning"],
    ["In Review", "info"],
    ["Accepted", "success"],
    ["Rejected", "destructive"],
    ["Superseded", "ignored"],
    ["Deprecated", "outline"],
  ])("draws %s as the package's %s badge", (status, variant) => {
    render(<StatusPill status={status} />);
    const badge = screen.getByText(status);
    expect(badge).toHaveAttribute("data-slot", "badge");
    expect(badge).toHaveAttribute("data-variant", variant);
  });

  it("draws an unknown status as the neutral outline badge", () => {
    render(<StatusPill status="Percolating" />);
    expect(screen.getByText("Percolating")).toHaveAttribute(
      "data-variant",
      "outline",
    );
  });

  it("is tinted, never filled, and sets no colour of its own", () => {
    render(<StatusPill status="Accepted" />);
    const badge = screen.getByText("Accepted");
    expect(badge).toHaveAttribute("data-fill", "tinted");
    expect(badge).not.toHaveAttribute("data-tone");
    expect(badge).not.toHaveAttribute("style");
  });
});

describe("StatusBadge", () => {
  it.each([
    ["Draft", "warning", "warn"],
    ["Proposed", "info", "info"],
    ["Implemented", "success", "ok"],
    ["Cancelled", "destructive", "bad"],
    ["Superseded", "ignored", "neutral"],
    ["Archived", "outline", "neutral"],
    ["Percolating", "outline", "neutral"],
  ])("draws %s as a label with a %s dot", (status, variant, tone) => {
    const { container } = render(<StatusBadge status={status} />);
    expect(screen.getByText(status)).toHaveAttribute("data-variant", variant);
    expect(dot(container)).toHaveAttribute("data-tone", tone);
  });

  it("hides the dot from assistive technology: the label is the status", () => {
    const { container } = render(<StatusBadge status="Accepted" />);
    expect(dot(container)).toHaveAttribute("aria-hidden", "true");
  });
});

describe("TypeDot", () => {
  it("takes a curated type's colour as a custom tone", () => {
    const { container } = render(<TypeDot type="rfc" />);
    expect(dot(container)).toHaveAttribute("data-tone", "custom");
    expect(dot(container).style.backgroundColor).toBe("var(--color-t-rfc)");
  });

  it("gives a custom type one of the eight category colours, stably", () => {
    const first = render(<TypeDot type="postmortem" />);
    const colour = dot(first.container).style.backgroundColor;
    expect(colour).toMatch(
      /^var\(--color-hash-[0-7], var\(--category-[1-8]\)\)$/,
    );
    const second = render(<TypeDot type="PostMortem" />);
    expect(dot(second.container).style.backgroundColor).toBe(colour);
  });
});

describe("TypeBadge", () => {
  it("is a tinted badge toned with the type's colour", () => {
    render(<TypeBadge type="design" />);
    const badge = screen.getByText("design");
    expect(badge).toHaveAttribute("data-slot", "badge");
    expect(badge).toHaveAttribute("data-tone", "custom");
    expect(badge).toHaveAttribute("data-fill", "tinted");
    expect(badge.style.getPropertyValue("--tone")).toBe(
      "var(--color-t-design)",
    );
  });

  it("takes a label in place of the type's name", () => {
    render(<TypeBadge type="investigation">INV</TypeBadge>);
    expect(screen.getByText("INV").style.getPropertyValue("--tone")).toBe(
      "var(--color-t-investigation)",
    );
  });
});
