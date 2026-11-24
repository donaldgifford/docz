import { buttonVariants } from "@donaldgifford/design-system/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@donaldgifford/design-system/components/card";
import { cn } from "@donaldgifford/design-system/lib/utils";
import type { ReactNode } from "react";
import { Link } from "react-router";

import { useGetSession } from "@/api/__generated__/docz-api";
import {
  enabledProviders,
  lastUsedProvider,
  promoteLastUsed,
  rememberProvider,
} from "@/lib/authProviders";
import { classifySession } from "@/lib/session";

/*
 * Provider selection page (DESIGN-0001 auth flow, IMPL-0001 Phase 5).
 * The buttons are REAL anchors to docz-api's /auth/login — the flow is
 * a full document navigation through the same-origin proxy (302 to the
 * provider), never a router transition. The last-used provider (a
 * localStorage UI preference) takes the primary slot; GitHub otherwise.
 *
 * None-mode (DESIGN-0003): when the shared session query classifies as
 * `anonymous` (provider "none"), the buttons give way to a quiet
 * auth-disabled panel — those /auth routes aren't mounted upstream.
 * Every other state, a still-pending probe included, renders the
 * buttons unchanged (OQ-3a: zero added latency for normal
 * deployments; a none-mode cold visit at worst flashes buttons that
 * 404 exactly as they do today).
 *
 * Each button is an anchor wearing the design system's button classes
 * (buttonVariants), not its Button with `render`: Base UI's Button gives
 * a rendered anchor the button role, and these must stay links, to the
 * browser and to a screen reader alike.
 */

function LoginCard({
  title,
  titleTestId,
  description,
  children,
}: {
  title: string;
  titleTestId?: string;
  description: ReactNode;
  children: ReactNode;
}) {
  return (
    <main className="mx-auto flex max-w-sm flex-col px-5 pt-[14vh]">
      <Card className="gap-5 py-6">
        <CardHeader className="px-6">
          <CardTitle
            // eslint-disable-next-line jsx-a11y/heading-has-content -- CardTitle hands the h1 its children
            render={<h1 data-testid={titleTestId} />}
            className="text-lg/none"
          >
            {title}
          </CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardContent className="px-6">{children}</CardContent>
      </Card>
    </main>
  );
}

export function Component() {
  const sessionQuery = useGetSession();
  const state = classifySession(sessionQuery);
  const lastUsed = lastUsedProvider();
  const providers = promoteLastUsed(enabledProviders(), lastUsed);

  if (state.kind === "anonymous") {
    return (
      <LoginCard
        title="Authentication is disabled"
        titleTestId="login-auth-disabled"
        description="This deployment runs without sign-in — everything here is readable without an account."
      >
        <Link
          to="/"
          className={cn(
            buttonVariants({ variant: "outline", size: "lg" }),
            "w-full",
          )}
        >
          Browse the docs
        </Link>
      </LoginCard>
    );
  }

  return (
    <LoginCard
      title="Sign in"
      description="Authentication is handled by docz-api — no tokens ever reach this app."
    >
      <ul className="flex flex-col gap-2">
        {providers.map((provider, index) => (
          <li key={provider.key}>
            <a
              href={`/auth/login?provider=${encodeURIComponent(provider.key)}`}
              data-testid={`login-${provider.key}`}
              onClick={() => {
                rememberProvider(provider.key);
              }}
              className={cn(
                buttonVariants({
                  // The first is the last one used, or GitHub.
                  variant: index === 0 ? "default" : "outline",
                  size: "lg",
                }),
                "w-full",
              )}
            >
              Continue with {provider.label}
              {provider.key === lastUsed && (
                <span className="opacity-80"> · last used</span>
              )}
            </a>
          </li>
        ))}
      </ul>
    </LoginCard>
  );
}
