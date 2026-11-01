/*
 * The widget fences, kept apart from the validator so the eager reader
 * bundle can recognise a block without loading the YAML parser, which
 * arrives with the lazy widget chunk (IMPL-0030 Phase 5).
 */
export type WidgetName = "flow" | "stepper" | "scorecard";

/** The fence info strings, mapped to the widget each draws. */
export const WIDGET_FENCES: Readonly<Record<string, WidgetName>> = {
  "docz-flow": "flow",
  "docz-stepper": "stepper",
  "docz-scorecard": "scorecard",
};
