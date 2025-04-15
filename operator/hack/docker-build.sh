#!/usr/bin/env bash
# /*
# Copyright 2025 The Grove Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# */


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