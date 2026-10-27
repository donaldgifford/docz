import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  acceptRecommendations,
  optionLabel,
  QuestionCards,
} from "@/review/questions";
import { INSPECT } from "@/review/testing";

import type { QuestionDraft } from "@/review/draft";

const QUESTIONS = INSPECT.open_questions;

function Harness({
  initial = {},
  onAnswers,
}: {
  initial?: Record<number, QuestionDraft>;
  onAnswers: (next: Record<number, QuestionDraft>) => void;
}) {
  const [answers, setAnswers] = useState(initial);
  return (
    <QuestionCards
      questions={QUESTIONS}
      answers={answers}
      onChange={(next) => {
        onAnswers(next);
        setAnswers(next);
      }}
    />
  );
}

function card(n: number) {
  const el = document.querySelector<HTMLElement>(
    `[data-question="${String(n)}"]`,
  );
  if (el === null) {
    throw new Error(`no card ${String(n)}`);
  }
  return el;
}

describe("QuestionCards", () => {
  it("starts every question unanswered with the recommendation badged", () => {
    const seen = vi.fn<(next: Record<number, QuestionDraft>) => void>();
    render(<Harness onAnswers={seen} />);
    for (const radio of screen.getAllByRole("radio")) {
      expect(radio).toHaveAttribute("aria-checked", "false");
    }
    expect(card(1)).toHaveTextContent("a. Valkey.recommended");
    expect(card(1)).not.toHaveTextContent("(recommendation)");
  });

  it("records a choice, Other text, and a note", async () => {
    const user = userEvent.setup();
    const seen = vi.fn<(next: Record<number, QuestionDraft>) => void>();
    render(<Harness onAnswers={seen} />);
    await user.click(within(card(1)).getByRole("radio", { name: /Memcached/ }));
    await user.click(within(card(2)).getByRole("radio", { name: "Other" }));
    await user.type(screen.getByLabelText("Other answer"), "a week");
    await user.type(within(card(1)).getByLabelText("Note"), "cheaper");
    expect(seen.mock.lastCall?.[0]).toEqual({
      1: { choice: "b", note: "cheaper" },
      2: { choice: "other", text: "a week" },
    });
  });

  it("accepts every recommendation only for unanswered questions", async () => {
    const user = userEvent.setup();
    const seen = vi.fn<(next: Record<number, QuestionDraft>) => void>();
    render(<Harness onAnswers={seen} initial={{ 2: { choice: "b" } }} />);
    await user.click(
      screen.getByRole("button", { name: "Accept all recommendations" }),
    );
    expect(seen.mock.lastCall?.[0]).toEqual({
      1: { choice: "a" },
      2: { choice: "b" },
    });
    expect(
      screen.getByRole("button", { name: "Accept all recommendations" }),
    ).toBeDisabled();
  });

  it("clears an answer back to unanswered", async () => {
    const user = userEvent.setup();
    const seen = vi.fn<(next: Record<number, QuestionDraft>) => void>();
    render(<Harness onAnswers={seen} initial={{ 1: { choice: "a" } }} />);
    await user.click(screen.getByRole("button", { name: "Clear answer" }));
    expect(seen.mock.lastCall?.[0]).toEqual({});
  });

  it("shows a resolved question read-only", () => {
    render(<Harness onAnswers={vi.fn()} />);
    expect(card(3)).toHaveTextContent("Resolved 2026-10-02: (a)");
    expect(card(3).querySelector("[role=radio]")).toBeNull();
  });
});

describe("helpers", () => {
  it("strips markdown and the marker from an option", () => {
    expect(optionLabel("**One hour.** *(recommendation)*")).toBe("One hour.");
  });

  it("leaves resolved questions alone", () => {
    expect(acceptRecommendations(QUESTIONS, {})).toEqual({
      1: { choice: "a" },
      2: { choice: "a" },
    });
  });
});
