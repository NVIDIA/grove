#!/usr/bin/env bash
# /*
# Copyright 2024 The Grove Authors.
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
BINARY_DIR="${OPERATOR_GO_MODULE_ROOT}/bin"

function build_ld_flags() {
  local package_path="github.com/NVIDIA/grove/operator/internal"
  local version="$(cat "${OPERATOR_GO_MODULE_ROOT}/VERSION")"
  local program_name="grove-operator"
  local build_date="$(date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$/:\1/g')"

  echo "-X $package_path/version.gitVersion=$version
        -X $package_path/version.gitCommit=$(git rev-parse --verify HEAD)
        -X $package_path/version.buildDate=$build_date
        -X $package_path/version.programName=$program_name"
}

function build_grove_operator() {
  local ld_flags=$(build_ld_flags)
  echo "> Building grove-operator with ldflags: $ld_flags ..."
  CGO_ENABLED=0 GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) GO111MODULE=on \
    go build \
    -o "${BINARY_DIR}/grove-operator" \
    -ldflags "${ld_flags}" \
    cmd/main.go
}

mkdir -p ${BINARY_DIR}
build_grove_operator

