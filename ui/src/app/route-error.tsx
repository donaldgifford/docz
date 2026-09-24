import { useEffect } from "react";
import { Link, isRouteErrorResponse, useRouteError } from "react-router";

import { ERROR_PANEL_ACTION, ErrorPanelFrame } from "@/components/query-states";

/*
 * Route error boundary (DESIGN-0006 Component 9).
 *
 * Before this, a render or lazy-load throw inside a route unmounted to
 * a blank page with nothing but a console message (INV-0006 F9).
 *
 * It CATCHES AND DISPLAYS — it does not export. No collector, no
 * beacon, no global window.onerror or unhandledrejection handler:
 * those are only useful if something records them, and that decision
 * belongs with browser telemetry rather than here.
 */

/** Best-effort description of whatever the router handed us. */
function describe(error: unknown): string {
  if (isRouteErrorResponse(error)) {
    return `${String(error.status)} ${error.statusText}`;
  }
  if (error instanceof Error) {
    // The raw message is what makes a report actionable, and it is the
    // app's own text — the same bargain ErrorPanel already makes for
    // API failures.
    return error.message;
  }
  return "This page failed to render.";
}

export function RouteErrorBoundary() {
  const error = useRouteError();

  useEffect(() => {
    // The boundary changes what the USER sees, not what a developer
    // can observe. Logged from an effect rather than in render so a
    // double-invoked render in StrictMode cannot double-log.
    console.error("route error boundary caught:", error);
  }, [error]);

  return (
    <ErrorPanelFrame message={describe(error)}>
      {/*
       * A link home is the ONLY affordance (OQ-5a). A "retry" that
       * re-rendered the same crashed route would simply throw again,
       * which reads to the user as a button that does nothing.
       */}
      <Link to="/" className={ERROR_PANEL_ACTION}>
        go home
      </Link>
    </ErrorPanelFrame>
  );
}
