#!/usr/bin/env bash
# Build + push every App Platform service image. Handy before the first deploy.
# Logs in once, then reuses the session for all five.
set -euo pipefail
here="$(dirname "${BASH_SOURCE[0]}")"
source "${here}/build-common.sh"

build_and_push "spade-web-ui"    "web_ui/Dockerfile"              "web_ui"
build_and_push "spade-migrate"   "web_ui/Dockerfile.migrate"      "web_ui"
build_and_push "spade-scheduler" "server/Dockerfile"              "."
build_and_push "spade-kms"       "kms/Dockerfile"                 "."
build_and_push "spade-registry"  "registry/Dockerfile.registryd"  "."

# Droplet-pulled images (not App Platform): the standalone build runner and the
# per-language builder images it launches. See build-buildrunner.sh /
# build-builder-images.sh to push these individually.
build_and_push "spade-buildrunner" "registry/Dockerfile.buildrunnerd" "."
for lang in go rust ts python r; do
  build_and_push "spade-builder-${lang}" "registry/Dockerfile.builder-${lang}" "."
done

echo ">> All images built and pushed at tag '${TAG}'."
