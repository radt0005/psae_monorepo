#!/usr/bin/env bash
# Build + push the web UI image (Nuxt/Bun) to DOCR.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/build-common.sh"
build_and_push "spade-web-ui" "web_ui/Dockerfile" "web_ui"
