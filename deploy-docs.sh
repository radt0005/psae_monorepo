#!/usr/bin/env bash
# Build the documentation site (documentation/, Zola) and deploy it to
# Cloudflare Pages. Cloudflare owns the static hosting end to end.
set -euo pipefail
cd "$(dirname "$0")"
source ./deploy-common.sh

build_and_deploy "documentation" "${DOCS_PAGES_PROJECT:-spade-docs}" "https://docs.psae.us"
