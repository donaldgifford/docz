import { Badge } from "@donaldgifford/design-system/components/badge";
import { Button } from "@donaldgifford/design-system/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@donaldgifford/design-system/components/card";
import { cn } from "@donaldgifford/design-system/lib/utils";
import type { ReactNode } from "react";
import { useSearchParams } from "react-router";

import {
  useDecideOAuthAuthorization,
  useGetOAuthAuthorization,
} from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import {
  EmptyPanel,
  ErrorPanel,
  InlineLoading,
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

/** The page's one column, around the card and around each state. */
function ConsentPage({ children }: { children: ReactNode }) {
  return (
    <main className="mx-auto flex max-w-md flex-col px-5 pt-[12vh]">
      {children}
    </main>
  );
}

function ScopeList({ authorization }: { authorization: OAuthAuthorization }) {
  const granted = new Set(arr(authorization.scopes_granted));
  const requested = arr(authorization.scopes_requested);
  return (
    <ul className="flex flex-col gap-2" data-testid="consent-scopes">
      {requested.map((scope) => {
        const given = granted.has(scope);
        return (
          <li
            key={scope}
            data-testid={`consent-scope-${given ? "granted" : "withheld"}`}
            className={cn(
              "flex flex-wrap items-center gap-2 rounded-md border px-3 py-2 text-sm",
              !given && "text-muted-foreground",
            )}
          >
            <Badge variant="outline">{scope}</Badge>
            <span className={cn(!given && "line-through")}>
              {describeScope(scope)}
            </span>
            {!given && (
              <span className="font-mono text-xs">
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
      <ConsentPage>
        <div data-testid="consent-missing">
          <EmptyPanel
            className="my-0"
            title="This request can't be answered"
            description="This authorization request is unknown, expired, or already answered. Start again from the application that sent you here."
          />
        </div>
      </ConsentPage>
    );
  }

  if (query.isError) {
    return (
      <ConsentPage>
        <ErrorPanel
          className="my-0"
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      </ConsentPage>
    );
  }

  if (query.data?.status !== 200) {
    return (
      <ConsentPage>
        <Card>
          <InlineLoading data-testid="consent-pending">
            Loading the request…
          </InlineLoading>
        </Card>
      </ConsentPage>
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
    <ConsentPage>
      <Card className="gap-5 py-6">
        <CardHeader className="px-6">
          <CardTitle
            // eslint-disable-next-line jsx-a11y/heading-has-content -- CardTitle hands the h1 its children
            render={<h1 />}
            className="text-lg/tight"
          >
            Authorize{" "}
            {authorization.client_uri !== undefined &&
            authorization.client_uri !== "" ? (
              <a
                href={authorization.client_uri}
                rel="noreferrer"
                target="_blank"
                data-testid="consent-client-link"
                className="text-primary underline underline-offset-4"
              >
                {authorization.client_name}
              </a>
            ) : (
              <span data-testid="consent-client-name">
                {authorization.client_name}
              </span>
            )}
            ?
          </CardTitle>
          <CardDescription>
            It is asking to act on your behalf with these permissions. The token
            it gets expires in minutes and renews only while you keep using it.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex flex-col gap-4 px-6">
          <ScopeList authorization={authorization} />

          {decide.isError && (
            <p role="alert" className="font-mono text-xs text-destructive">
              {decide.error instanceof Error
                ? decide.error.message
                : "The decision could not be recorded"}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="lg"
              disabled={busy}
              onClick={() => {
                submit("deny");
              }}
            >
              Deny
            </Button>
            <Button
              size="lg"
              disabled={busy}
              onClick={() => {
                submit("allow");
              }}
            >
              Allow
            </Button>
          </div>
          <p className="font-mono text-xs text-muted-foreground">
            client: {authorization.client_id}
          </p>
        </CardContent>
      </Card>
    </ConsentPage>
  );
}
