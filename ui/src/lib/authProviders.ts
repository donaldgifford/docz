/*
 * Login providers (DESIGN-0001 "Auth and session handling", IMPL-0001
 * Phase 5): docz-api owns the whole OAuth/OIDC exchange; the site only
 * needs to know which provider buttons to render. There is no
 * discovery endpoint, so the deployment declares them via
 * VITE_AUTH_PROVIDERS (comma-separated, baked at build time) — GitHub
 * alone by default, mirroring docz-api's AUTH_PROVIDERS default.
 */

export interface AuthProvider {
  key: string;
  label: string;
}

// The three providers docz-api implements (its /auth/login rejects
// anything else with a 400).
const KNOWN_PROVIDERS: Readonly<Record<string, string | undefined>> = {
  github: "GitHub",
  okta: "Okta",
  keycloak: "Keycloak",
};

const DEFAULT_PROVIDERS = "github";

export function parseProviders(raw: string | undefined): AuthProvider[] {
  const keys = [
    ...new Set(
      (raw ?? DEFAULT_PROVIDERS)
        .split(",")
        .map((key) => key.trim().toLowerCase())
        .filter((key) => key !== ""),
    ),
  ];
  const known = keys.flatMap((key) => {
    const label = KNOWN_PROVIDERS[key];
    return label === undefined ? [] : [{ key, label }];
  });
  // A misconfigured list must never brick the login page.
  return known.length > 0 ? known : [{ key: "github", label: "GitHub" }];
}

/*
 * Runtime config injected into index.html by the production server
 * (server/serve.ts reads DOCZ_AUTH_PROVIDERS). This is how a Helm
 * deployment picks providers without rebuilding the image; it takes
 * precedence over the build-time VITE_AUTH_PROVIDERS default.
 */
declare global {
  interface Window {
    __DOCZ_CONFIG__?: { authProviders?: string[] };
  }
}

function runtimeProviders(): string | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const list = window.__DOCZ_CONFIG__?.authProviders;
  return Array.isArray(list) && list.length > 0 ? list.join(",") : undefined;
}

export function enabledProviders(): AuthProvider[] {
  return parseProviders(
    runtimeProviders() ??
      (import.meta.env.VITE_AUTH_PROVIDERS as string | undefined),
  );
}

/*
 * Last-used provider — a localStorage UI preference (the only kind of
 * thing allowed in JS-readable storage; never a token). The login page
 * promotes it to the primary slot on the next visit.
 */

const LAST_PROVIDER_KEY = "docz:auth:last-provider";

export function rememberProvider(key: string): void {
  try {
    localStorage.setItem(LAST_PROVIDER_KEY, key);
  } catch {
    // Storage disabled — the preference just doesn't stick.
  }
}

export function lastUsedProvider(): string | null {
  try {
    return localStorage.getItem(LAST_PROVIDER_KEY);
  } catch {
    return null;
  }
}

/** Moves the remembered provider to the front; no match, no change. */
export function promoteLastUsed(
  providers: AuthProvider[],
  lastKey: string | null,
): AuthProvider[] {
  const match = providers.find((provider) => provider.key === lastKey);
  if (match === undefined) {
    return providers;
  }
  return [match, ...providers.filter((provider) => provider !== match)];
}
