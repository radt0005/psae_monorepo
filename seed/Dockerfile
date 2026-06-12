# Local block-seeding service.
#
# This is the *local-development substitute for the cloud Plugin Registry*
# (spec/registry.md).  In production, the registry screens/builds/signs
# collections and pushes their metadata into Postgres; workers fetch signed
# artifacts on demand.  None of that exists yet, so for local testing this
# one-shot service performs the two populations the registry would otherwise
# drive, directly from the in-repo collection sources:
#
#   1. Worker block index — `spade install` builds each Rust collection and
#      registers it in the SQLite block index on the shared worker volume
#      (/var/spade), so the worker can actually execute blocks.
#   2. Editor mirror — each block manifest is upserted into the Postgres
#      `blocks` table the web UI reads for its block picker / connection
#      resolver (web_ui/server/db/schema/blocks.ts).
#
# Scope: base + data (both Rust).  GDAL (Python) and sae (R) are a later pass.
#
# The build context must be the *monorepo root* so this image can reach
# core/, cli/, blocks/, and seed/ (cli/go.mod has `replace core => ../core`).

# --------------------------------------------------------------------------
# Stage 1: build the spade CLI (provides `spade install`).
# --------------------------------------------------------------------------
FROM golang:1.25-bookworm AS cli-build
WORKDIR /src

COPY core ./core
COPY cli ./cli

WORKDIR /src/cli
RUN go mod download
# CGO is required: the block registry uses mattn/go-sqlite3 (cgo). The
# bookworm image ships gcc; go-sqlite3 links SQLite statically.
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/spade .

# --------------------------------------------------------------------------
# Stage 2: seed runtime — Rust toolchain (to build collections) plus the
# psql/yq tooling used to mirror manifests into Postgres.
# --------------------------------------------------------------------------
FROM rust:1-bookworm AS seed
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        postgresql-client ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

# yq (mikefarah) — YAML → JSON for the manifest mirror step.
RUN curl -fsSL \
        https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 \
        -o /usr/local/bin/yq \
    && chmod +x /usr/local/bin/yq

# uv — builds the Python (gdal) collection's environment.  uv fetches a
# managed Python 3.12 itself, so no system python3.12 is required here.
RUN curl -LsSf https://astral.sh/uv/install.sh | HOME=/root sh \
    && ln -sf /root/.local/bin/uv /usr/local/bin/uv

COPY --from=cli-build /out/spade /usr/local/bin/spade

# Runtime libraries the collections depend on via relative paths
# (`spade = { path = "../../libs/rust" }` and `../../libs/python`).  Must sit
# at /seed/libs/<lang> so the `../../` from /seed/blocks/<c> resolves here.
COPY libs/rust /seed/libs/rust
COPY libs/python /seed/libs/python

# Block collection sources.  Pre-compiling the Rust collections here (at
# image-build time) means the runtime `spade install` finds an up-to-date
# target/ and is a fast register-only no-op rather than a multi-minute cargo
# build on every `docker compose up`.
COPY blocks/base /seed/blocks/base
COPY blocks/data /seed/blocks/data
RUN cargo build --release --manifest-path /seed/blocks/base/Cargo.toml \
    && cargo build --release --manifest-path /seed/blocks/data/Cargo.toml

# Python (gdal) collection source.  Built at runtime into the shared volume
# (its venv carries absolute paths that must match the worker's mount), so it
# is only staged here for the seed script to copy onto the volume.
COPY blocks/gdal /seed/blocks/gdal

COPY seed/seed.sh /usr/local/bin/seed.sh
RUN chmod +x /usr/local/bin/seed.sh

ENTRYPOINT ["/usr/local/bin/seed.sh"]
