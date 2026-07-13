#!/usr/bin/env bash
# Shared build+push helper for Spade service images, sourced by the per-service
# build-<service>.sh scripts. Not meant to be run directly.
#
# Env overrides:
#   TAG                 image tag to build/push        (default: latest)
#   SPADE_REGISTRY      DOCR path                       (default: registry.digitalocean.com/spade-default-1)
#   PLATFORM            target arch for App Platform    (default: linux/amd64)
#   SKIP_REGISTRY_LOGIN set to 1 to skip doctl login    (default: run it once)
set -euo pipefail

TAG="${TAG:-latest}"
SPADE_REGISTRY="${SPADE_REGISTRY:-registry.digitalocean.com/spade-default-1}"
PLATFORM="${PLATFORM:-linux/amd64}"

# Repo root = the directory this file lives in, so scripts work from any cwd.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Authenticate the local Docker daemon to DOCR once per shell (idempotent).
# Needs a doctl token with registry read/write; set SKIP_REGISTRY_LOGIN=1 if
# you've already logged in.
if [[ "${SKIP_REGISTRY_LOGIN:-0}" != "1" && -z "${_SPADE_REGISTRY_LOGGED_IN:-}" ]]; then
  echo ">> doctl registry login"
  doctl registry login
  export _SPADE_REGISTRY_LOGGED_IN=1
fi

# build_and_push <image-name> <dockerfile-relpath> <context-relpath>
build_and_push() {
  local image="$1"
  local dockerfile="$2"
  local context="$3"
  local ref="${SPADE_REGISTRY}/${image}:${TAG}"

  echo ">> Building ${ref}"
  echo "     dockerfile: ${dockerfile}"
  echo "     context:    ${context}"
  echo "     platform:   ${PLATFORM}"
  docker build \
    --platform "${PLATFORM}" \
    -f "${REPO_ROOT}/${dockerfile}" \
    -t "${ref}" \
    "${REPO_ROOT}/${context}"

  echo ">> Pushing ${ref}"
  docker push "${ref}"

  echo ">> Done: ${ref}"
}
