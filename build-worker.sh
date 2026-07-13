#!/usr/bin/env bash
# Build + push the worker image (Go + isolate + language runtimes) to DOCR.
# Not an App Platform service — pulled by the worker Droplets' cloud-init.
# Build context is the repo root (go.mod uses replace directives to ../core).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/build-common.sh"
build_and_push "spade-worker" "runner/worker/Dockerfile" "."
