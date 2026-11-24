import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LifecycleRail, TocList } from "@/components/doc-rail";

import type { TocEntry } from "@/markdown/processor";

/*
 * The reader's side parts are the design system's Outline and Stepper;
 * these hold what docz hands them. For the outline that is the page's
 * headings and the scroll spy's band: jsdom has no IntersectionObserver
 * and computes no layout, so the observer is faked and the tests drive
 * its callback (the band's geometry is the browser's, and the e2e sweep
 * covers it). For the lifecycle it is where a document's status puts
 * each of its type's statuses.
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

  constructor(
    private readonly callback: (entries: Entry[]) => void,
    readonly options?: IntersectionObserverInit,
  ) {
    FakeObserver.instances.push(this);
  }
  observe(el: Element): void {
    this.observed.push(el);
  }
  unobserve(): void {
    /* not used */
  }
  disconnect(): void {
    /* not used */
  }
  /** Test driver: report a set of headings as in/out of the band. */
  fire(entries: Entry[]): void {
    act(() => {
      this.callback(entries);
    });
  }
}

function renderOutline(toc: TocEntry[] = TOC) {
  return render(
    <>
      <article>
        {toc.map((entry) => (
          <h2 key={entry.id} id={entry.id}>
            {entry.text}
          </h2>
        ))}
      </article>
      <TocList toc={toc} />
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

describe("TocList", () => {
  it("lists the page's headings as links to them, under its own label", () => {
    renderOutline();

    expect(
      screen.getByRole("navigation", { name: "On this page" }),
    ).toBeInTheDocument();
    expect(TOC.map((entry) => link(entry.text).getAttribute("href"))).toEqual([
      "#first",
      "#nested",
      "#second",
    ]);
  });

  it("draws nothing for a page with no headings", () => {
    const { container } = renderOutline([]);
    expect(container.querySelector("nav")).toBeNull();
  });

  it("watches every heading in a band that clears the top bar", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderOutline();

    const observer = FakeObserver.instances[0];
    expect(observer?.observed).toEqual(TOC.map((entry) => heading(entry.id)));
    expect(observer?.options?.rootMargin).toBe("-80px 0px -70% 0px");
  });

  it("marks no heading before one reaches the band", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderOutline();

    for (const entry of TOC) {
      expect(link(entry.text)).not.toHaveAttribute("aria-current");
    }
  });

  it("marks the heading in the band as the current location", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderOutline();

    FakeObserver.instances[0]?.fire([
      { target: heading("nested"), isIntersecting: true },
    ]);

    expect(link("Nested")).toHaveAttribute("aria-current", "location");
    expect(link("First")).not.toHaveAttribute("aria-current");
  });

  it("keeps the topmost heading current when several are in the band", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderOutline();

    FakeObserver.instances[0]?.fire([
      { target: heading("second"), isIntersecting: true },
      { target: heading("first"), isIntersecting: true },
    ]);

    // Document order wins over callback order.
    expect(link("First")).toHaveAttribute("aria-current", "location");
  });

  it("holds the last heading current once it scrolls out of the band", () => {
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    renderOutline();
    const observer = FakeObserver.instances[0];

    observer?.fire([{ target: heading("first"), isIntersecting: true }]);
    // A long section fills the screen: nothing is in the band now.
    observer?.fire([{ target: heading("first"), isIntersecting: false }]);

    expect(link("First")).toHaveAttribute("aria-current", "location");
  });

  it("renders without a spy where IntersectionObserver is missing", () => {
    // jsdom's default: the outline must draw, not throw.
    expect(globalThis).not.toHaveProperty("IntersectionObserver");
    renderOutline();

    expect(link("First")).not.toHaveAttribute("aria-current");
  });
});

/** docz-api's design type in the fixtures: five statuses, in order. */
function renderLifecycle(currentStatus: string, typeName = "design") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={queryClient}>
      <LifecycleRail
        owner="donaldgifford"
        name="docz-api"
        typeName={typeName}
        currentStatus={currentStatus}
      />
    </QueryClientProvider>,
  );
  return { ...view, queryClient };
}

/** Each step as [label, status], once the disclosure is open. */
async function openSteps(): Promise<[string | null, string | undefined][]> {
  await userEvent.click(
    await screen.findByRole("button", { name: /^Lifecycle/ }),
  );
  return screen
    .getAllByRole("listitem")
    .map((step) => [step.textContent, step.dataset.status]);
}

describe("LifecycleRail", () => {
  it("is closed until asked, and says the status in force", async () => {
    renderLifecycle("Approved");

    expect(
      await screen.findByRole("button", { name: /^Lifecycle\s*Approved$/ }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
  });

  it("puts the statuses before the document's behind it, and the rest ahead", async () => {
    renderLifecycle("Approved");

    expect(await openSteps()).toEqual([
      ["Draft", "done"],
      ["In Review", "done"],
      ["Approved", "current"],
      ["Implemented", "upcoming"],
      ["Abandoned", "upcoming"],
    ]);
    expect(
      screen
        .getAllByRole("listitem")
        .filter((step) => step.getAttribute("aria-current") === "step")
        .map((step) => step.textContent),
    ).toEqual(["Approved"]);
  });

  it("matches the document's status whatever its case", async () => {
    renderLifecycle("in review");

    const steps = await openSteps();
    expect(steps[1]).toEqual(["In Review", "current"]);
  });

  it.each([
    ["no status", "", /^Lifecycle\s*unset$/],
    ["a status its type does not declare", "Shelved", /^Lifecycle\s*Shelved$/],
  ])(
    "reaches no step for a document with %s",
    async (_, status, triggerName) => {
      renderLifecycle(status);

      expect(
        await screen.findByRole("button", { name: triggerName }),
      ).toBeInTheDocument();
      const steps = await openSteps();
      expect(steps.map(([, state]) => state)).toEqual([
        "upcoming",
        "upcoming",
        "upcoming",
        "upcoming",
        "upcoming",
      ]);
    },
  );

  it("draws nothing, not even its box, for a type the repo does not declare", async () => {
    const { container, queryClient } = renderLifecycle("Draft", "nonsense");

    // Read the absence once listTypes has answered, not before.
    expect(queryClient.isFetching()).toBe(1);
    await waitFor(() => {
      expect(queryClient.isFetching()).toBe(0);
    });
    expect(container).toBeEmptyDOMElement();
  });
});
