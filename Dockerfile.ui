# Multi-stage Dockerfile for ShitWiz UI
# Supports both development (hot reload) and production (optimized static build with Caddy)

# =============================================================================
# Base stage: Node.js + Bun
# =============================================================================
FROM node:20-alpine AS base

# Install Bun globally
RUN npm install -g bun

# Set working directory for all subsequent stages
WORKDIR /app

# =============================================================================
# Development stage: Hot reload for local development
# =============================================================================
FROM base AS dev

# Copy dependency files (lockfile is optional)
COPY package.json ./
COPY bun.lock* ./

# Install all dependencies (including devDependencies)
RUN bun install

# Copy application code
COPY . .

# Expose Vite dev server port
EXPOSE 3000

# Start Vite dev server with host binding for Docker
CMD ["bun", "run", "dev", "--host"]

# =============================================================================
# Builder stage: Production build
# =============================================================================
FROM base AS builder

# Copy dependency files (lockfile is optional)
COPY package.json ./
COPY bun.lock* ./

# Install dependencies
RUN bun install

# Copy application code
COPY . .

# Build optimized production bundle
RUN bun run build

# =============================================================================
# Production stage: Caddy static file server
# =============================================================================
FROM caddy:2-alpine AS production

# Copy built static files from builder stage
COPY --from=builder /app/dist /srv

# Copy Caddy configuration
COPY Caddyfile /etc/caddy/Caddyfile

# Copy entrypoint script for runtime config injection
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Expose HTTP port
EXPOSE 80

# Use entrypoint for runtime config, then start Caddy
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
