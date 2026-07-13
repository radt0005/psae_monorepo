#!/usr/bin/env bash
# Build + push the five per-language builder images to DOCR. Not App Platform
# services — the build-runner Droplet pulls these and the buildrunnerd
# dispatcher launches one ephemeral container per build (registry.md §5.2).
# Locally, docker-compose builds the same Dockerfiles instead (the one-shot
# builder-*-image services). Build context is the repo root.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/build-common.sh"

for lang in go rust ts python r; do
  build_and_push "spade-builder-${lang}" "registry/Dockerfile.builder-${lang}" "."
done
