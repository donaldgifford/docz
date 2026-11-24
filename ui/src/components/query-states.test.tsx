import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  EmptyPanel,
  ErrorPanel,
  NotFoundPanel,
  ProseSkeleton,
} from "@/components/query-states";

/*
 * The panels are the design system's blocks; these hold what docz adds
 * to them: its wording, what a rejection that is not an Error reads as,
 * and that a retry is offered only where one was asked for.
 */

describe("NotFoundPanel", () => {
  it("keeps the deliberately ambiguous wording: a 404 also hides a private repo", () => {
    render(<NotFoundPanel />);
    expect(
      screen.getByRole("heading", {
        name: "Not found — or not visible to you",
      }),
    ).toBeInTheDocument();
  });
});

describe("EmptyPanel", () => {
  it("draws the way out it is given", () => {
    render(
      <EmptyPanel
        title="No matches"
        action={<button type="button">clear filters</button>}
      />,
    );
    expect(screen.getByText("No matches")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "clear filters" }),
    ).toBeInTheDocument();
  });
});

describe("ErrorPanel", () => {
  it("shows an Error's own message under the fixed heading", () => {
    render(<ErrorPanel error={new Error("docz-api returned 502")} />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Something went wrong");
    expect(alert).toHaveTextContent("docz-api returned 502");
  });

  it.each([
    ["a thrown string", "boom"],
    ["a plain object", { status: 500 }],
    ["nothing at all", undefined],
  ])("reads %s as the caller's fallback, never its String()", (_, thrown) => {
    render(<ErrorPanel error={thrown} fallback="Search failed" />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Search failed");
    expect(alert).not.toHaveTextContent("[object Object]");
    expect(alert).not.toHaveTextContent("undefined");
  });

  it("defaults the fallback to Request failed", () => {
    render(<ErrorPanel error={null} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Request failed");
  });

  it("offers a retry only when it is handed one", async () => {
    const onRetry = vi.fn();
    const { rerender } = render(<ErrorPanel error={new Error("x")} />);
    expect(
      screen.queryByRole("button", { name: "Retry" }),
    ).not.toBeInTheDocument();

    rerender(<ErrorPanel error={new Error("x")} onRetry={onRetry} />);
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("draws another recovery under the panel, outside the alert", () => {
    render(
      <ErrorPanel error={new Error("x")}>
        <a href="/">go home</a>
      </ErrorPanel>,
    );
    const link = screen.getByRole("link", { name: "go home" });
    expect(screen.getByRole("alert")).not.toContainElement(link);
  });
});

describe("ProseSkeleton", () => {
  it("is hidden from assistive technology and found by its test id", () => {
    render(<ProseSkeleton data-testid="page-skeleton" lines={3} />);
    const skeleton = screen.getByTestId("page-skeleton");
    expect(skeleton).toHaveAttribute("aria-hidden", "true");
    // A title and the three lines.
    expect(skeleton.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(4);
  });
});
