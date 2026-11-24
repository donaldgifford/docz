import { EmptyState } from "@donaldgifford/design-system/blocks/empty-state";
import { ErrorState } from "@donaldgifford/design-system/blocks/error-state";
import { InlineLoading } from "@donaldgifford/design-system/blocks/inline-state";
import { Skeleton } from "@donaldgifford/design-system/components/skeleton";
import { cn } from "@donaldgifford/design-system/lib/utils";
import { IconSearch } from "@tabler/icons-react";
import { useEffect, type ComponentProps, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router";

import { stashReturnTo } from "@/lib/authReturn";

/*
 * Shared fetch-state panels (DESIGN-0001 "States" table, DESIGN-0028
 * §7). The reader, directory, and repo pages all funnel their loading,
 * empty, and error states through here, keyed off the typed errors from
 * src/api/fetcher.ts. Each panel is one of the design system's blocks;
 * what this file owns is docz's wording, where a panel sits on a page,
 * and the rule each wrapper's comment records.
 */

/** Where a state panel sits when it stands in for a page's content. */
const PANEL = "mx-auto my-16 max-w-xl";

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

/**
 * Nothing to show, and what to do about it: an empty list, a filter
 * with no matches. `action` is the way out, when there is one.
 */
export function EmptyPanel({
  className,
  ...props
}: ComponentProps<typeof EmptyState>) {
  return <EmptyState className={cn(PANEL, className)} {...props} />;
}

/** Neutral 404 — deliberately ambiguous; 404 also hides private repos. */
export function NotFoundPanel() {
  return (
    <EmptyPanel
      icon={IconSearch}
      title="Not found — or not visible to you"
      description="The address may be wrong, or the repository may not be shared with you."
    />
  );
}

/**
 * The design system's ErrorState reads its message from an Error. A
 * rejection that is not one (a thrown string, an aborted request's
 * reason) would print as "[object Object]", so it takes the caller's
 * wording instead.
 */
function asError(error: unknown, fallback: string): Error {
  return error instanceof Error ? error : new Error(fallback);
}

/**
 * A failed load: what went wrong, and a way to recover. `children` is
 * the recovery when it is not a retry, drawn under the panel: the link
 * out of a crashed route in RouteErrorBoundary, where asking again
 * would only throw again.
 */
export function ErrorPanel({
  error,
  fallback = "Request failed",
  onRetry,
  children,
  className,
}: {
  error: unknown;
  /** The message for a rejection that is not an Error. */
  fallback?: string;
  onRetry?: () => void;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn(PANEL, "flex flex-col items-center gap-3", className)}>
      <ErrorState
        className="w-full"
        title="Something went wrong"
        error={asError(error, fallback)}
        onRetry={onRetry}
      />
      {children}
    </div>
  );
}

/*
 * Loading has no panel of docz's own. A page draws the design system's
 * Skeleton blocks where its content will be (hidden from assistive
 * technology, with a data-testid, since a pulse has no text to find),
 * and a list or a card that is waiting says so with InlineLoading, which
 * is a status region. Both are re-exported so a route takes every state
 * from this one module.
 */
export { InlineLoading, Skeleton };

/**
 * A page of prose while its first request is out: a title and lines of
 * text. `data-testid` names it for tests, since a pulse has no text.
 */
export function ProseSkeleton({
  lines = 6,
  ...props
}: {
  lines?: number;
  "data-testid": string;
}) {
  return (
    <div aria-hidden {...props}>
      <Skeleton className="mb-6 h-8 w-2/5" />
      <div className="space-y-3">
        {Array.from({ length: lines }, (_, i) => (
          <Skeleton
            key={i}
            className="h-3"
            style={{ width: `${String(96 - (i % 3) * 12)}%` }}
          />
        ))}
      </div>
    </div>
  );
}
