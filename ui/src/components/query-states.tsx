/*
 * Shared fetch-state panels (DESIGN-0001 "States" table). The reader,
 * directory, and repo pages all funnel their error handling through
 * these three, keyed off the typed errors from src/api/fetcher.ts.
 */

/** Bare 401 panel — docz-api owns the whole login flow (post-MVP UX). */
export function SessionRequiredPanel() {
  return (
    <div className="mx-auto my-16 w-max max-w-full border border-border-default bg-bg-raised px-8 py-6 text-center">
      <p className="font-mono text-[13px] text-fg-secondary">
        Session required
      </p>
      <p className="mt-2 text-[13px] text-fg-tertiary">
        <a
          href="/auth/login?provider=github"
          className="text-accent hover:underline"
        >
          Sign in with GitHub
        </a>{" "}
        to read documents.
      </p>
    </div>
  );
}

/** Neutral 404 — deliberately ambiguous; 404 also hides private repos. */
export function NotFoundPanel() {
  return (
    <div className="mx-auto my-16 w-max max-w-full border border-border-default bg-bg-raised px-8 py-6 text-center">
      <p className="font-mono text-[13px] text-fg-secondary">
        Not found — or not visible to you
      </p>
    </div>
  );
}

export function ErrorPanel({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div className="mx-auto my-16 w-max max-w-full border border-[color-mix(in_srgb,var(--color-st-rejected)_35%,transparent)] bg-bg-raised px-8 py-6 text-center">
      <p className="font-mono text-[13px] text-fg-secondary">
        Something went wrong
      </p>
      <p className="mt-2 max-w-96 text-[13px] text-fg-tertiary">{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-4 border border-border-strong px-4 py-1 font-mono text-[12px] text-fg-secondary hover:bg-bg-hover"
      >
        retry
      </button>
    </div>
  );
}
