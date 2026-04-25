# syntax=docker/dockerfile:1.7

# ---- Build stage ------------------------------------------------------------
# Bun is required at runtime: server/db/index.ts uses drizzle-orm/bun-sql
# (Bun.sql is a native Bun API, not available under Node).

FROM oven/bun:1 AS build

WORKDIR /app

# Install deps first so the layer is cached when only source changes.
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source and build the Nuxt app for the Bun preset.
COPY . .
ENV NITRO_PRESET=bun
RUN bun --bun run build


# ---- Runtime stage ----------------------------------------------------------

FROM oven/bun:1-slim AS runtime

WORKDIR /app

# Nitro packs everything the server needs into .output (incl. node_modules
# subset). Migrations stay outside .output so we can run drizzle-kit migrate
# from the same image.
COPY --from=build /app/.output ./.output
COPY --from=build /app/server/db/migrations ./server/db/migrations
COPY --from=build /app/drizzle.config.ts ./drizzle.config.ts

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000
EXPOSE 3000

# To run pending migrations against the configured DATABASE_URL:
#   docker run --rm -e DATABASE_URL=... <image> bunx drizzle-kit migrate
ENTRYPOINT ["bun", "run", ".output/server/index.mjs"]
