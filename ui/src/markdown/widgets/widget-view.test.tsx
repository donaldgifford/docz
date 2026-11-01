import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

/*
 * The widgets through the real pipeline (IMPL-0030 Phase 5): a fence is
 * marked, MarkdownPre routes it, the lazy chunk validates and draws.
 */
async function mount(md: string, review = false): Promise<HTMLElement> {
  const { content } = await renderMarkdown(md, { review });
  const { container } = render(<>{content}</>);
  return container;
}

const FLOW = [
  "Intro.",
  "",
  "```docz-flow",
  "title: Ingest",
  "stages:",
  "  - {name: Webhook, does: A push lands.}",
  "  - {name: Queue, does: A job waits., note: Debounced.}",
  "gates: [2]",
  "alert: Retries five times.",
  "```",
  "",
].join("\n");

const STEPPER = [
  "```docz-stepper",
  "steps:",
  "  - {key: auth, name: Authenticate, does: The token is checked.}",
  "  - {key: scope, name: Authorize, does: Scopes are matched.}",
  "  - {key: serve, name: Serve, does: The document is returned.}",
  "scenarios:",
  "  - {name: Allowed}",
  "  - name: Denied",
  "    fails_at: scope",
  "    notes: {scope: The token lacks the scope.}",
  "```",
  "",
].join("\n");

const SCORECARD = [
  "```docz-scorecard",
  "checks:",
  "  - {id: C1, name: Config, target: present, weight: 2}",
  "  - {id: C2, name: Clean, target: zero findings}",
  "example:",
  "  subject: acme/docs",
  "  results:",
  "    - {id: C1, score: 1, observed: present}",
  "    - {id: C2, score: 0.5}",
  "```",
  "",
].join("\n");

describe("widgets in the reader", () => {
  it("draws a flow and shows a stage's detail", async () => {
    const container = await mount(FLOW);
    const figure = await waitFor(() => {
      const f = container.querySelector('figure[data-widget="flow"]');
      expect(f).not.toBeNull();
      return f as HTMLElement;
    });
    expect(within(figure).getByText("Ingest").tagName).toBe("FIGCAPTION");
    expect(within(figure).getByText("Retries five times.")).toBeDefined();
    const queue = within(figure).getByRole("button", { name: /Queue/ });
    expect(queue.textContent).toContain("(gate)");
    await userEvent.click(queue);
    expect(queue.getAttribute("aria-pressed")).toBe("true");
    expect(within(figure).getByText("Debounced.")).toBeDefined();
  });

  it("walks a stepper scenario to the step it fails at", async () => {
    const container = await mount(STEPPER);
    const figure = await waitFor(() => {
      const f = container.querySelector('figure[data-widget="stepper"]');
      expect(f).not.toBeNull();
      return f as HTMLElement;
    });
    await userEvent.click(
      within(figure).getByRole("button", { name: "Denied" }),
    );
    await userEvent.click(
      within(figure).getByRole("button", { name: /Authorize/ }),
    );
    expect(
      within(figure).getByText("Denied", { selector: ".adm-label" }),
    ).toBeDefined();
    expect(
      within(figure).getByText("The token lacks the scope."),
    ).toBeDefined();
    await userEvent.click(within(figure).getByRole("button", { name: "Next" }));
    expect(
      within(figure).getByText(/Never reached: stopped at step 2/),
    ).toBeDefined();
  });

  it("scores a scorecard's example", async () => {
    const container = await mount(SCORECARD);
    const figure = await waitFor(() => {
      const f = container.querySelector('figure[data-widget="scorecard"]');
      expect(f).not.toBeNull();
      return f as HTMLElement;
    });
    await userEvent.click(
      within(figure).getByRole("button", { name: "Load the example" }),
    );
    // (1 × 2 + 0.5 × 1) / 3 = 83%.
    expect(within(figure).getByText("2.5 / 3.0")).toBeDefined();
    expect(
      within(figure).getByRole("meter").getAttribute("aria-valuenow"),
    ).toBe("83");
    expect(within(figure).getByText("acme/docs")).toBeDefined();
  });

  it("says why pasted results are not JSON", async () => {
    const container = await mount(SCORECARD);
    const figure = await waitFor(() => {
      const f = container.querySelector('figure[data-widget="scorecard"]');
      expect(f).not.toBeNull();
      return f as HTMLElement;
    });
    await userEvent.type(
      within(figure).getByLabelText("Results as JSON"),
      "nope",
    );
    await userEvent.click(
      within(figure).getByRole("button", { name: "Score" }),
    );
    expect(within(figure).getByText(/^Not valid JSON/)).toBeDefined();
  });

  it("shows an invalid block's errors at document lines, then its YAML", async () => {
    // The fence opens on line 3, so the YAML's first line is line 4.
    const container = await mount(
      "Intro.\n\n```docz-flow\nstages:\n  - {name: a, does: b, colour: red}\n```\n",
    );
    const invalid = await waitFor(() => {
      const d = container.querySelector(".widget-invalid");
      expect(d).not.toBeNull();
      return d as HTMLElement;
    });
    expect(within(invalid).getByText("Invalid docz-flow block")).toBeDefined();
    expect(
      [...invalid.querySelectorAll("li")].map((li) => li.textContent),
    ).toEqual(["line 5: field colour not found"]);
    expect(invalid.querySelector("pre code")?.textContent).toContain(
      "colour: red",
    );
    expect(container.querySelector("figure.widget")).toBeNull();
  });

  it("counts lines from the document, frontmatter included, in review", async () => {
    const container = await mount(
      "---\nid: X\n---\n\n```docz-flow\nstages: abc\n```\n",
      true,
    );
    const item = await screen.findByText(/cannot unmarshal/);
    expect(item.textContent).toBe("line 6: cannot unmarshal !!str `abc`");
    expect(
      container.querySelector(".widget-invalid")?.getAttribute("data-line"),
    ).toBe("5");
  });

  it("keeps the review stamp on a drawn widget", async () => {
    const container = await mount(FLOW, true);
    const figure = await waitFor(() => {
      const f = container.querySelector('figure[data-widget="flow"]');
      expect(f).not.toBeNull();
      return f as HTMLElement;
    });
    expect(figure.getAttribute("data-line")).toBe("3");
  });
});
