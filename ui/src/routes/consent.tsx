import { useSearchParams } from "react-router";

import {
  useDecideOAuthAuthorization,
  useGetOAuthAuthorization,
} from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import {
  ERROR_PANEL_ACTION,
  ErrorPanel,
  ErrorPanelFrame,
  SessionRequiredRedirect,
} from "@/components/query-states";
import { arr } from "@/lib/wire";

import type { OAuthAuthorization } from "@/api/__generated__/docz-api.schemas";

/*
 * The OAuth consent page (DESIGN-0023 §5, IMPL-0026 Phase 4). docz-api's
 * `/oauth/authorize` sends the person here with `?authz=<id>`; the page
 * reads the pending authorization through the gated consent endpoint,
 * shows the client and the scopes it will get, and posts the decision.
 * The API answers with where to send the browser (the client's
 * redirect_uri with a code or access_denied), and the page sets
 * window.location to it — a full navigation out of the app, never a
 * router transition.
 *
 * Behind the session gate like every route: a 401 stashes this route
 * (query included) in docz:auth:return-to and lands on /login, and the
 * shell restores it once the provider round trip is back on "/"
 * (IMPL-0026 question 4). The id is the only secret the page handles,
 * and docz-api refuses it from any session but the first that read it.
 */

/** The reader-facing words for a scope (`<audience>:<action>`). */
export function describeScope(scope: string): string {
  const [audience, action] = scope.split(":", 2);
  if (audience === undefined || action === undefined) {
    return scope;
  }
  const verb = action === "read" ? "Read" : action;
  return audience === "public"
    ? `${verb} every repository that is public to signed-in people`
    : `${verb} the repositories shared with "${audience}"`;
}

function ConsentCard({ children }: { children: React.ReactNode }) {
  return (
    <main className="mx-auto flex max-w-[480px] flex-col px-5 pt-[12vh]">
      <div className="border border-border-default bg-bg-raised px-8 py-8">
        <div className="mb-1 font-mono text-[13.5px] tracking-[0.05em] text-accent">
          / docz <span className="text-fg-muted">/</span> authorize
        </div>
        {children}
      </div>
    </main>
  );
}

function ScopeList({ authorization }: { authorization: OAuthAuthorization }) {
  const granted = new Set(arr(authorization.scopes_granted));
  const requested = arr(authorization.scopes_requested);
  return (
    <ul className="mb-6 flex flex-col gap-2" data-testid="consent-scopes">
      {requested.map((scope) => {
        const given = granted.has(scope);
        return (
          <li
            key={scope}
            data-testid={`consent-scope-${given ? "granted" : "withheld"}`}
            className={`border px-3 py-2 text-[14px] ${
              given
                ? "border-border-default text-fg-secondary"
                : "border-border-hairline text-fg-muted line-through"
            }`}
          >
            <span className="font-mono text-[12.5px] text-accent">{scope}</span>
            <span className="mx-2 text-fg-muted">·</span>
            {describeScope(scope)}
            {!given && (
              <span className="ml-2 font-mono text-[11.5px] no-underline">
                (you don&apos;t hold this)
              </span>
            )}
          </li>
        );
      })}
    </ul>
  );
}

export function Component() {
  const [params] = useSearchParams();
  const id = params.get("authz") ?? "";
  const query = useGetOAuthAuthorization(id, {
    query: { enabled: id !== "", retry: false },
  });
  const decide = useDecideOAuthAuthorization();

  if (query.error instanceof SessionRequiredError) {
    return <SessionRequiredRedirect />;
  }

  if (id === "" || query.error instanceof NotFoundError) {
    return (
      <ConsentCard>
        <ErrorPanelFrame message="This authorization request is unknown, expired, or already answered. Start again from the application that sent you here.">
          <span data-testid="consent-missing" />
        </ErrorPanelFrame>
      </ConsentCard>
    );
  }

  if (query.isError) {
    return (
      <ConsentCard>
        <ErrorPanel
          message={
            query.error instanceof Error
              ? query.error.message
              : "Request failed"
          }
          onRetry={() => {
            void query.refetch();
          }}
        />
      </ConsentCard>
    );
  }

  if (query.data?.status !== 200) {
    return (
      <ConsentCard>
        <p
          data-testid="consent-pending"
          className="font-mono text-[13px] text-fg-muted"
        >
          loading…
        </p>
      </ConsentCard>
    );
  }

  const authorization = query.data.data;
  const submit = (decision: "allow" | "deny") => {
    decide.mutate(
      { id, data: { decision } },
      {
        onSuccess: (result) => {
          if (result.status === 200) {
            window.location.assign(result.data.redirect_to);
          }
        },
      },
    );
  };
  const busy = decide.isPending || decide.isSuccess;

  return (
    <ConsentCard>
      <h1 className="mb-2 text-[22px] font-semibold tracking-[-0.01em] text-fg-primary">
        Authorize{" "}
        {authorization.client_uri !== undefined &&
        authorization.client_uri !== "" ? (
          <a
            href={authorization.client_uri}
            rel="noreferrer"
            target="_blank"
            data-testid="consent-client-link"
            className="text-accent underline decoration-(--color-accent-border) underline-offset-4"
          >
            {authorization.client_name}
          </a>
        ) : (
          <span data-testid="consent-client-name">
            {authorization.client_name}
          </span>
        )}
        ?
      </h1>
      <p className="mb-5 text-[14px] text-fg-tertiary">
        It is asking to act on your behalf with these permissions. The token it
        gets expires in minutes and renews only while you keep using it.
      </p>

      <ScopeList authorization={authorization} />

      {decide.isError && (
        <p
          role="alert"
          className="mb-4 font-mono text-[13px] text-(--color-st-rejected)"
        >
          {decide.error instanceof Error
            ? decide.error.message
            : "The decision could not be recorded"}
        </p>
      )}

      <div className="flex gap-3">
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            submit("allow");
          }}
          className="flex-1 border border-(--color-accent-border) bg-(--color-accent-bg) px-4 py-[0.55rem] font-mono text-[14px] text-accent hover:border-accent disabled:opacity-60"
        >
          Allow
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            submit("deny");
          }}
          className={`flex-1 ${ERROR_PANEL_ACTION} mt-0`}
        >
          Deny
        </button>
      </div>
      <p className="mt-4 font-mono text-[12px] text-fg-muted">
        client: {authorization.client_id}
      </p>
    </ConsentCard>
  );
}
