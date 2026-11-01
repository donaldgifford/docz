import type { ReactNode } from "react";

import { defaultTheme } from "@/lib/theme";

import "@/theme/__generated__/page.css";

/**
 * accentProperties makes a type's colour a document's accent
 * (DESIGN-0026 §8). accent is typeColor()'s var(--color-t-…) reference,
 * set on the .docz-page itself so it resolves against the document
 * theme's own --color-t-* values, which contrast.test.ts holds to AA on
 * that theme's surfaces, rather than the shell's. Only the accent moves:
 * links keep the theme's link colour, which is what keeps a link in body
 * text apart from the text around it (axe link-in-text-block), and the
 * tints stay the theme's.
 */
export function accentProperties(accent: string): Record<string, string> {
  return {
    "--color-accent": accent,
  };
}

/**
 * DocSurface is the document theme around markdown a page renders itself
 * (DESIGN-0026 §3): in the reader a published page, the repo home, the
 * changelog, a type's index, and a document whose fragment did not load;
 * on the review page the document column, in the repository's theme.
 * It is the .docz-page a fragment arrives in, so page.css gives it the
 * deployment's document theme and the export's type, and the shell's look
 * stops at its edge.
 */
export function DocSurface({
  children,
  accent,
  theme = defaultTheme(),
}: {
  children: ReactNode;
  accent?: string;
  /** The document theme; the deployment's when omitted. */
  theme?: string;
}) {
  return (
    <div
      className="docz-page rounded-sm px-8 py-6"
      data-theme={theme}
      data-testid="doc-surface"
      style={accent === undefined ? undefined : accentProperties(accent)}
    >
      {children}
    </div>
  );
}
