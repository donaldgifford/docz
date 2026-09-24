import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TocList } from "@/components/doc-rail";

import type { TocEntry } from "@/markdown/processor";

/*
 * The scroll spy behind the outline rail. jsdom has no
 * IntersectionObserver and computes no layout, so the observer is
 * faked: the tests drive its callback directly, which is the only part
 * of the contract that belongs to us (the band geometry is the
 * browser's job and is covered by the e2e sweep).
 */

const TOC: TocEntry[] = [
  { depth: 2, text: "First", id: "first" },
  { depth: 3, text: "Nested", id: "nested" },
  { depth: 2, text: "Second", id: "second" },
];

type Entry = Pick<IntersectionObserverEntry, "isIntersecting"> & {
  target: Element;
};

class FakeObserver {
  static instances: FakeObserver[] = [];
  readonly observed: Element[] = [];
  disconnected = false;

  constructor(private readonly callback: (entries: Entry[]) => void) {
    FakeObserver.instances.push(this);
  }
  observe(el: Element): void {
    this.observed.push(el);
  }
  unobserve(): void {
    /* not used */
  }
  disconnect(): void {
    this.disconnected = true;
  }
  /** Test driver: report a set of headings as in/out of the band. */
  fire(entries: Entry[]): void {
    act(() => {
      this.callback(entries);
    });
  }
}

function renderRail() {
  return render(
    <>
      <article>
        {TOC.map((entry) => (
          <h2 key={entry.id} id={entry.id}>
            {entry.text}
          </h2>
        ))}
      </article>
      <TocList toc={TOC} />
    </>,
  );
}

function link(name: string): HTMLElement {
  return screen.getByRole("link", { name });
}

/** The rendered heading element; the observer reports these as targets. */
function heading(id: string): HTMLElement {
  const el = document.getElementById(id);
  if (el === null) {
    throw new Error(`heading #${id} is missing from the document`);
  }
  return el;
}

afterEach(() => {
  FakeObserver.instances = [];
  vi.unstubAllGlobals();
});

describe("useActiveHeading", () => {
  it("marks no row current before a heading reaches the band", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderRail();

    for (const entry of TOC) {
      expect(link(entry.text)).not.toHaveAttribute("aria-current");
    }
  });

  it("marks the heading in the band as the current location", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderRail();
    const observer = FakeObserver.instances[0];
    expect(observer?.observed).toHaveLength(TOC.length);

    observer?.fire([{ target: heading("nested"), isIntersecting: true }]);

    expect(link("Nested")).toHaveAttribute("aria-current", "location");
    expect(link("First")).not.toHaveAttribute("aria-current");
  });

  it("keeps the topmost band heading current when several are visible", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderRail();
    const observer = FakeObserver.instances[0];

    observer?.fire([
      { target: heading("second"), isIntersecting: true },
      { target: heading("first"), isIntersecting: true },
    ]);

    // Document order wins over callback order.
    expect(link("First")).toHaveAttribute("aria-current", "location");
  });

  it("holds the last heading current once it scrolls out of the band", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderRail();
    const observer = FakeObserver.instances[0];

    observer?.fire([{ target: heading("first"), isIntersecting: true }]);
    // A long section fills the screen: nothing is in the band now.
    observer?.fire([{ target: heading("first"), isIntersecting: false }]);

    expect(link("First")).toHaveAttribute("aria-current", "location");
  });

  it("renders without a spy where IntersectionObserver is missing", () => {
    // jsdom's default: the hook must no-op, not throw.
    expect(globalThis).not.toHaveProperty("IntersectionObserver");
    renderRail();

    expect(link("First")).not.toHaveAttribute("aria-current");
  });
});
