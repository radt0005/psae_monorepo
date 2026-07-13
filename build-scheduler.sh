#!/usr/bin/env bash
# Build + push the scheduler image (Go). Build context is the repo root.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/build-common.sh"
build_and_push "spade-scheduler" "server/Dockerfile" "."
