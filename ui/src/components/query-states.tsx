import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router";

import { stashReturnTo } from "@/lib/authReturn";

/*
 * Shared fetch-state panels (DESIGN-0001 "States" table). The reader,
 * directory, and repo pages all funnel their error handling through
 * these three, keyed off the typed errors from src/api/fetcher.ts.
 */

/**
 * 401 → `/login`, stashing the intended destination so the OAuth round
 * trip (which always lands back on "/") can restore it. Replaced the
 * MVP-era "session required" panel — same call sites, Phase 5 behavior.
 */
export function SessionRequiredRedirect() {
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    stashReturnTo(location.pathname + location.search);
    void navigate("/login", { replace: true });
  }, [location.pathname, location.search, navigate]);

  // Nothing to paint — the redirect commits on the next tick.
  return null;
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
