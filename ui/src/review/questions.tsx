import { Badge } from "@donaldgifford/design-system/components/badge";
import { useId } from "react";

import { FIELD_CLASS, LABEL_CLASS, ReviewButton } from "@/review/controls";

import type { Question } from "@/cli/__generated__/inspect";
import type { QuestionDraft } from "@/review/draft";

/*
 * Open-question cards (DESIGN-0024 section 5). Every question starts
 * unanswered: the recommended option is badged, never preselected, and
 * "accept all recommendations" fills only the unanswered ones, and only
 * when asked. A resolved question is read-only and shows its
 * resolution. An unanswered card sends nothing.
 */

/** An option is a button as wide as its card, whose text may wrap. */
const OPTION_CLASS =
  "h-auto min-h-(--control-h) justify-start py-1 text-left whitespace-normal";

type Answers = Record<number, QuestionDraft>;

/** An option's text without its markdown and the recommendation marker. */
export function optionLabel(text: string): string {
  return text
    .replace(/\*?\(recommendation\)\*?/gi, "")
    .replace(/[*_`]/g, "")
    .trim();
}

/** Fills every unanswered open question that has a recommended option. */
export function acceptRecommendations(
  questions: readonly Question[],
  answers: Answers,
): Answers {
  const next = { ...answers };
  for (const q of questions) {
    const recommended = q.options.find((o) => o.recommended);
    if (q.resolved !== null || recommended === undefined) {
      continue;
    }
    const current = next[q.number];
    if (current?.choice === undefined || current.choice === "") {
      next[q.number] = { ...current, choice: recommended.letter };
    }
  }
  return next;
}

function QuestionCard({
  question,
  answer,
  onChange,
}: {
  question: Question;
  answer: QuestionDraft | undefined;
  onChange: (next: QuestionDraft | undefined) => void;
}) {
  const id = useId();
  const resolved = question.resolved;
  const choice = answer?.choice;
  const set = (patch: Partial<QuestionDraft>) => {
    onChange({ ...answer, ...patch });
  };

  return (
    <section
      aria-labelledby={`${id}-title`}
      data-question={question.number}
      className="rounded-lg border bg-card p-3"
    >
      <h3 id={`${id}-title`} className="text-base font-medium text-foreground">
        Q{question.number}. {question.title}
      </h3>
      {resolved !== null ? (
        <p className="mt-2 font-mono text-xs text-muted-foreground">
          Resolved {resolved.date}: ({resolved.choice})
          {resolved.note === "" ? "" : ` ${resolved.note}`}
        </p>
      ) : (
        <>
          <div
            role="radiogroup"
            aria-labelledby={`${id}-title`}
            className="mt-2 flex flex-col gap-1"
          >
            {question.options.map((o) => (
              <ReviewButton
                key={o.letter}
                role="radio"
                aria-checked={choice === o.letter}
                variant={choice === o.letter ? "selected" : "plain"}
                className={OPTION_CLASS}
                onClick={() => {
                  set({ choice: o.letter });
                }}
              >
                {o.letter}. {optionLabel(o.text)}
                {o.recommended && (
                  <Badge variant="info" className="ml-auto">
                    recommended
                  </Badge>
                )}
              </ReviewButton>
            ))}
            <ReviewButton
              role="radio"
              aria-checked={choice === "other"}
              variant={choice === "other" ? "selected" : "plain"}
              className={OPTION_CLASS}
              onClick={() => {
                set({ choice: "other" });
              }}
            >
              Other
            </ReviewButton>
          </div>
          {choice === "other" && (
            <label className={`mt-2 ${LABEL_CLASS}`}>
              Other answer
              <textarea
                className={FIELD_CLASS}
                value={answer?.text ?? ""}
                onChange={(e) => {
                  set({ text: e.target.value });
                }}
              />
            </label>
          )}
          <label className={`mt-2 ${LABEL_CLASS}`}>
            Note
            <input
              className={FIELD_CLASS}
              value={answer?.note ?? ""}
              onChange={(e) => {
                set({ note: e.target.value });
              }}
            />
          </label>
          {choice !== undefined && (
            <ReviewButton
              size="sm"
              className="mt-2"
              onClick={() => {
                onChange(undefined);
              }}
            >
              Clear answer
            </ReviewButton>
          )}
        </>
      )}
    </section>
  );
}

export function QuestionCards({
  questions,
  answers,
  onChange,
}: {
  questions: readonly Question[];
  answers: Answers;
  onChange: (next: Answers) => void;
}) {
  if (questions.length === 0) {
    return null;
  }
  const canAccept = questions.some(
    (q) =>
      q.resolved === null &&
      q.options.some((o) => o.recommended) &&
      (answers[q.number]?.choice ?? "") === "",
  );
  return (
    <div className="flex flex-col gap-3" aria-label="Open questions">
      <ReviewButton
        disabled={!canAccept}
        onClick={() => {
          onChange(acceptRecommendations(questions, answers));
        }}
      >
        Accept all recommendations
      </ReviewButton>
      {questions.map((q) => (
        <QuestionCard
          key={q.number}
          question={q}
          answer={answers[q.number]}
          onChange={(next) => {
            const rest = Object.fromEntries(
              Object.entries(answers).filter(([n]) => Number(n) !== q.number),
            );
            onChange(next === undefined ? rest : { ...rest, [q.number]: next });
          }}
        />
      ))}
    </div>
  );
}
