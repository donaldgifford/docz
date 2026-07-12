import {
  enabledProviders,
  lastUsedProvider,
  promoteLastUsed,
  rememberProvider,
} from "@/lib/authProviders";

/*
 * Provider selection page (DESIGN-0001 auth flow, IMPL-0001 Phase 5).
 * The buttons are REAL anchors to docz-api's /auth/login — the flow is
 * a full document navigation through the same-origin proxy (302 to the
 * provider), never a router transition. The last-used provider (a
 * localStorage UI preference) takes the primary slot; GitHub otherwise.
 */

export function Component() {
  const lastUsed = lastUsedProvider();
  const providers = promoteLastUsed(enabledProviders(), lastUsed);

  return (
    <main className="mx-auto flex max-w-[420px] flex-col px-5 pt-[14vh]">
      <div className="border border-border-default bg-bg-raised px-8 py-8">
        <div className="mb-1 font-mono text-[12.5px] tracking-[0.05em] text-accent">
          / docz <span className="text-fg-muted">/</span> sign in
        </div>
        <h1 className="mb-2 text-[22px] font-semibold tracking-[-0.01em] text-fg-primary">
          Sign in
        </h1>
        <p className="mb-6 text-[13px] text-fg-tertiary">
          Authentication is handled by docz-api — no tokens ever reach this app.
        </p>

        <ul className="flex flex-col gap-2">
          {providers.map((provider, index) => (
            <li key={provider.key}>
              <a
                href={`/auth/login?provider=${encodeURIComponent(provider.key)}`}
                data-testid={`login-${provider.key}`}
                onClick={() => {
                  rememberProvider(provider.key);
                }}
                className={`block border px-4 py-[0.55rem] text-center font-mono text-[13px] ${
                  index === 0
                    ? "border-(--color-accent-border) bg-(--color-accent-bg) text-accent hover:border-accent"
                    : "border-border-default text-fg-secondary hover:border-border-strong hover:text-fg-primary"
                }`}
              >
                Continue with {provider.label}
                {provider.key === lastUsed && (
                  <span className="text-fg-muted"> · last used</span>
                )}
              </a>
            </li>
          ))}
        </ul>
      </div>
    </main>
  );
}
