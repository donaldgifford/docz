/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Set by the dev:msw script — serve the API from MSW fixtures. */
  readonly VITE_API_MODE?: "msw";
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
