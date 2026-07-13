#!/usr/bin/env bash
# Build the marketing site (website/, Zola) and deploy it to Cloudflare Pages.
# Cloudflare owns the static hosting end to end (CDN + TLS + custom domain).
set -euo pipefail
cd "$(dirname "$0")"
source ./deploy-common.sh

build_and_deploy "website" "${WEBSITE_PAGES_PROJECT:-spade-website}" "https://spade.psae.us"
