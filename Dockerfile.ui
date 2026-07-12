# Multi-stage build (IMPL-0001 Phase 4): full toolchain to build,
# slim Bun runtime to serve. The runtime image contains only dist/ and
# server/serve.ts — no node_modules, no sources.
#
#   docker build -t docz-site .
#   docker run -p 8080:8080 -e DOCZ_API_URL=http://docz-api:8080 docz-site

FROM oven/bun:1.3.14 AS build
WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . .
# The orval client is generated, never committed (src/api/__generated__
# is gitignored) — regenerate from the vendored spec before building.
RUN bun run gen-api && bun run build

# Precompress the text assets where it pays; fonts/images are already
# compressed. serve.ts prefers the .gz sibling when the client accepts.
RUN find dist -type f \( -name '*.js' -o -name '*.css' -o -name '*.svg' -o -name '*.html' -o -name '*.json' \) -exec gzip -k -9 {} \;

FROM oven/bun:1.3.14-slim
WORKDIR /app
ENV NODE_ENV=production

COPY --from=build /app/dist ./dist
COPY server/serve.ts ./server/serve.ts

USER bun
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD ["bun", "-e", "const r = await fetch('http://localhost:8080/healthz'); if (!r.ok) process.exit(1);"]

CMD ["bun", "server/serve.ts"]
