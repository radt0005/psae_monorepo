#!/usr/bin/env bash
# Build + push the DB migration image (runs as the App Platform PRE_DEPLOY job).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/build-common.sh"
build_and_push "spade-migrate" "web_ui/Dockerfile.migrate" "web_ui"
