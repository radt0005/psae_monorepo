#!/usr/bin/env bash
# Shared helper for building a Zola site and deploying it to Cloudflare Pages.
# Sourced by deploy-website.sh / deploy-docs.sh.
#
# Requires: zola, and wrangler (Cloudflare CLI). Authenticate wrangler once with
#   wrangler login            # interactive
# or export a scoped token for non-interactive use:
#   export CLOUDFLARE_API_TOKEN=...  CLOUDFLARE_ACCOUNT_ID=...
#
# By default wrangler runs via npx; override how it's invoked with e.g.
#   export WRANGLER="bunx wrangler"
set -euo pipefail

WRANGLER="${WRANGLER:-npx --yes wrangler}"

# build_and_deploy <site_dir> <pages_project> <public_url>
build_and_deploy() {
  local site_dir="$1" project="$2" url="$3"

  command -v zola >/dev/null || { echo "zola not found — install Zola (https://www.getzola.org)" >&2; return 1; }

  echo "==> Building $site_dir (zola build)"
  ( cd "$site_dir" && zola build )

  echo "==> Deploying $site_dir/public -> Cloudflare Pages project '$project'"
  $WRANGLER pages deploy "$site_dir/public" --project-name "$project"

  echo "==> Done. Live at $url"
}
