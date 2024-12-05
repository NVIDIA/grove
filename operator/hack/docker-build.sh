#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
OPERATOR_GO_MODULE_ROOT="$(dirname "$SCRIPT_DIR")"
GOARCH=${GOARCH:-$(go env GOARCH)}
PLATFORM=${PLATFORM:-linux/${GOARCH}}

function build_docker_image() {
  local version="$(cat "${OPERATOR_GO_MODULE_ROOT}/VERSION")"
  printf '%s\n' "Building grove-operator:${version} with:
   PLATFORM: ${PLATFORM}... "
  docker buildx build \
    --platform ${PLATFORM} \
    --build-arg VERSION=${version} \
    -t grove-operator-${GOARCH}:${version} \
    -f ${OPERATOR_GO_MODULE_ROOT}/Dockerfile \
    ${OPERATOR_GO_MODULE_ROOT}
}

build_docker_image