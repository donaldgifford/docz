/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Set by the dev:msw script — serve the API from MSW fixtures. */
  readonly VITE_API_MODE?: "msw";
  /**
   * Build-time diagram layout fallback, `dagre` or `elk`. Typed as a
   * plain string because it is untrusted input like any other env var —
   * `src/lib/mermaidLayout.ts` validates it. The runtime
   * DOCZ_MERMAID_LAYOUT wins over this.
   */
  readonly VITE_MERMAID_LAYOUT?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
