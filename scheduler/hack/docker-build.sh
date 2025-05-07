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
MODULE_ROOT="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(dirname "$MODULE_ROOT")"
GOARCH=${GOARCH:-$(go env GOARCH)}
PLATFORM=${PLATFORM:-linux/${GOARCH}}

function build_docker_image() {
  local version="$(cat "${MODULE_ROOT}/VERSION")"
  printf '%s\n' "Building scheduler-plugin:${version} with:
   PLATFORM: ${PLATFORM}... "
  docker buildx build \
    --platform ${PLATFORM} \
    --build-arg VERSION=${version} \
    --tag kube-scheduler:${version} \
    --file ${MODULE_ROOT}/Dockerfile \
    $REPO_ROOT # docker context is as the repository root to access `.git/`
}

build_docker_image
