# Multi-stage build for the spade-worker binary.
#
# Stage 1 compiles the binary; stage 2 packages it into an Ubuntu 24.04
# image that includes the isolate sandbox, language runtimes, and supporting
# tools required by the worker spec (`spec/worker.md`).
#
# The build context must be the *monorepo root* (`docker build -f
# runner/worker/Dockerfile .`) because go.mod uses `replace` directives
# that point at the sibling `../core` module.
#
# IMPORTANT — sandbox at runtime:
#   isolate requires kernel-level cgroup and user-namespace access.
#   Run this container with `privileged: true` in docker-compose.yml (see
#   the root docker-compose.yml).  For integration tests that do not need
#   real block execution, set SPADE_SKIP_ISOLATE_CHECK=1 instead.

FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY core ./core
COPY runner ./runner

WORKDIR /src/runner
RUN go mod download
# CGO must be enabled: the worker's registry uses mattn/go-sqlite3, a cgo
# library. golang:1.25-bookworm ships gcc; go-sqlite3 statically links SQLite,
# so the runtime image needs only glibc (present in ubuntu:24.04).
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/spade-worker ./cmd/spade-worker

# Ubuntu 24.04 LTS — isolate requires a real Ubuntu kernel environment.
FROM ubuntu:24.04 AS runtime
ARG DEBIAN_FRONTEND=noninteractive

# Language runtimes and isolate build dependencies.
# build-essential brings gcc + libc6-dev (needed for stdint.h in isolate).
# libsystemd-dev is required by isolate's Makefile (mirrors install_isolate.sh).
# libcap2 / libseccomp2 / libsystemd stay at runtime (isolate links them).
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl git \
        build-essential pkg-config libcap-dev libseccomp-dev libsystemd-dev \
        python3 \
        r-base \
    && rm -rf /var/lib/apt/lists/*

# Build and install isolate from source.
RUN git clone https://github.com/ioi/isolate.git /tmp/isolate \
    && cd /tmp/isolate \
    && make isolate \
    && make install \
    && chmod u+s /usr/local/bin/isolate \
    && mkdir -p /var/local/lib/isolate \
    && rm -rf /tmp/isolate

# subuid/subgid ranges isolate needs for user-namespace sandboxing.
RUN useradd --system --no-create-home isolate \
    && echo "isolate:100000:65536" >> /etc/subuid \
    && echo "isolate:100000:65536" >> /etc/subgid

# Install uv (Python package manager used by Python blocks).
RUN curl -LsSf https://astral.sh/uv/install.sh | HOME=/root sh \
    && ln -sf /root/.local/bin/uv /usr/local/bin/uv

# Install Bun (TypeScript/JS runtime for TypeScript blocks).
RUN curl -fsSL https://bun.sh/install | HOME=/root bash \
    && ln -sf /root/.bun/bin/bun /usr/local/bin/bun

COPY --from=build /out/spade-worker /usr/local/bin/spade-worker

RUN mkdir -p /var/spade/work /var/spade/registry /var/spade/cache

ENV SPADE_WORK_ROOT=/var/spade/work
ENV SPADE_REGISTRY=/var/spade/registry/registry.db
ENV SPADE_CACHE_DIR=/var/spade/cache

ENTRYPOINT ["/usr/local/bin/spade-worker"]
