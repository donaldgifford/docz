import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// docz-api serves no CORS headers — prod is same-origin (DESIGN-0001
// Decision 9). Dev mirrors that by proxying every path the SPA touches
// to a locally running docz-api (HTTP_ADDR defaults to :8080).
const doczApiUrl = process.env.DOCZ_API_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": doczApiUrl,
      "/auth": doczApiUrl,
      "/openapi.yaml": doczApiUrl,
    },
  },
});
