#!/usr/bin/env bash
# Build + push the standalone build-runner image (Go + docker CLI). Not an App
# Platform service — pulled by the build-runner Droplet's cloud-init, which runs
# it with the host docker socket mounted so it can launch builder containers.
# Build context is the repo root.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/build-common.sh"
build_and_push "spade-buildrunner" "registry/Dockerfile.buildrunnerd" "."
