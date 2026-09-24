import axe from "axe-core";
import { expect } from "vitest";

/*
 * Shared axe assertion for component tests (IMPL-0001 Phase 4): core
 * views must produce zero serious/critical violations. jsdom has no
 * layout or paint, so color-contrast can't run here — token contrast
 * is enforced mathematically in src/theme/contrast.test.ts and the
 * full rule set (contrast included) runs in the Playwright e2e suite
 * against a real browser.
 */

const JSDOM_UNSUPPORTED_RULES = ["color-contrast"];

export async function expectNoAxeViolations(
  container: Element = document.body,
): Promise<void> {
  const results = await axe.run(container, {
    rules: Object.fromEntries(
      JSDOM_UNSUPPORTED_RULES.map((rule) => [rule, { enabled: false }]),
    ),
  });
  const blocking = results.violations.filter(
    (violation) =>
      violation.impact === "serious" || violation.impact === "critical",
  );
  const report = blocking
    .map(
      (violation) =>
        `[${violation.impact ?? "?"}] ${violation.id}: ${violation.help}\n` +
        violation.nodes
          .map((node) => `  ${node.html}\n  ${node.failureSummary ?? ""}`)
          .join("\n"),
    )
    .join("\n\n");
  expect(blocking, report).toEqual([]);
}
