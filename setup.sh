#!/usr/bin/env bash
#
# setup.sh — automate the post-toolchain steps from SETUP.md.
#
# Assumes the language toolchains (rust, go, python, uv, R, bun) and the
# system GDAL library are already installed and on PATH. Run this from
# anywhere; it operates relative to the script's own directory.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_ROOT"

log()  { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

require() {
    command -v "$1" >/dev/null 2>&1 \
        || fail "'$1' not found on PATH. Install it first (see SETUP.md) and re-run."
}

log "Checking required toolchains"
require cargo
require go
require python3
require uv
require Rscript
require bun
if ! command -v gdalinfo >/dev/null 2>&1; then
    warn "gdalinfo not found — 'spade install blocks/gdal' will fail without system GDAL."
fi

log "Building and installing the spade CLI"
(cd cli && go install .)

GOBIN="$(go env GOBIN)"
[ -n "$GOBIN" ] || GOBIN="$(go env GOPATH)/bin"
if ! command -v spade >/dev/null 2>&1; then
    if [ -x "$GOBIN/spade" ]; then
        warn "spade installed to $GOBIN but that directory is not on PATH."
        warn "Add it with:  export PATH=\"$GOBIN:\$PATH\""
        export PATH="$GOBIN:$PATH"
    else
        fail "spade was not produced by 'go install'."
    fi
fi
spade --version 2>/dev/null || spade --help >/dev/null

log "Initializing ~/.spade/ via 'spade setup'"
spade setup

log "Compiling Rust block collections (base, data)"
cargo build --release --manifest-path blocks/base/Cargo.toml
cargo build --release --manifest-path blocks/data/Cargo.toml

log "Installing first-party block collections"
for collection in base data gdal ml sae; do
    log "spade install blocks/$collection"
    spade install "blocks/$collection"
done

log "Validating installed manifests"
spade check || warn "'spade check' reported issues — review the output above."

log "Setup complete. Installed collections:"
ls -1 "$HOME/.spade/blocks/" 2>/dev/null || warn "~/.spade/blocks/ is empty or missing."
