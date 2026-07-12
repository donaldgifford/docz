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

export function enabledProviders(): AuthProvider[] {
  return parseProviders(
    import.meta.env.VITE_AUTH_PROVIDERS as string | undefined,
  );
}
