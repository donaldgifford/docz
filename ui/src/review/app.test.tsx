import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { createReviewClient } from "@/review/api";
import { ReviewApp } from "@/review/app";
import { FakeReviewServer } from "@/review/fake-server";
import { INSPECT, reviewDocument, TOKEN } from "@/review/testing";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(() => Promise.reject(new Error("jsdom"))),
  },
}));

function mount() {
  const server = new FakeReviewServer({
    token: TOKEN,
    documents: [reviewDocument()],
  });
  const client = createReviewClient(TOKEN, server.fetch);
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ReviewApp client={client} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return server;
}

function selectWord(word: string) {
  const strong = Array.from(document.querySelectorAll("article strong")).find(
    (s) => s.textContent === word,
  );
  const text = strong?.firstChild;
  if (text === null || text === undefined) {
    throw new Error(`no ${word}`);
  }
  const range = document.createRange();
  range.setStart(text, 0);
  range.setEnd(text, word.length);
  document.getSelection()?.removeAllRanges();
  document.getSelection()?.addRange(range);
  act(() => {
    document.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
  });
}

describe("ReviewApp", () => {
  it("renders the round's document, findings, and questions", async () => {
    mount();
    expect(
      await screen.findByRole("heading", {
        name: "A cache for the reader",
        level: 1,
      }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("We use", { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Findings" })).toBeInTheDocument();
    expect(screen.getByText(/Q1\. Which cache\?/)).toBeInTheDocument();
    expect(
      document.querySelector('section[data-region="goals"]'),
    ).not.toBeNull();
  });

  it("submits one annotation with no question decisions", async () => {
    const user = userEvent.setup();
    const server = mount();
    await screen.findByText("Postgres");
    selectWord("Postgres");
    const popover = await screen.findByRole("dialog", {
      name: "Annotate selection",
    });
    await user.click(within(popover).getByRole("button", { name: "Comment" }));
    await user.type(
      within(popover).getByLabelText("Comment"),
      "Why not SQLite?",
    );
    await user.click(within(popover).getByRole("button", { name: "Add" }));
    await user.click(screen.getByRole("button", { name: "Submit" }));
    expect(
      await screen.findByText(/This round is closed: changes/),
    ).toBeInTheDocument();
    expect(server.records).toHaveLength(1);
    expect(server.records[0]).toMatchObject({
      outcome: "changes",
      questions: [],
      annotations: [
        {
          order: 1,
          kind: "comment",
          text: "Why not SQLite?",
          anchor: {
            exact: "Postgres",
            region: "goals",
            resolution: "exact",
            line: 14,
          },
        },
      ],
    });
  });

  it("disables Approve while a replace is drafted", async () => {
    const user = userEvent.setup();
    mount();
    await screen.findByText("Postgres");
    expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled();
    selectWord("Postgres");
    const popover = await screen.findByRole("dialog", {
      name: "Annotate selection",
    });
    await user.click(within(popover).getByRole("button", { name: "Replace" }));
    await user.type(within(popover).getByLabelText("Replace with"), "SQLite");
    await user.click(within(popover).getByRole("button", { name: "Add" }));
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Discard draft and approve" }),
    ).toBeInTheDocument();
  });

  it("keeps the draft and says so when the file changed", async () => {
    const user = userEvent.setup();
    const server = mount();
    await screen.findByText("Postgres");
    await user.click(screen.getByRole("radio", { name: /Memcached/ }));
    server.edit("DESIGN-0042", { ...INSPECT, content_sha256: "b".repeat(64) });
    await user.click(screen.getByRole("button", { name: "Submit" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /changed on disk/,
    );
    expect(screen.getByRole("radio", { name: /Memcached/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(server.records).toEqual([]);
  });
});
