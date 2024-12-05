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

source $(dirname $0)/ld-flags.sh

BINARY_DIR="${OPERATOR_GO_MODULE_ROOT}/bin"
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}

function build_grove_operator() {
  local ld_flags=$(build_ld_flags)
  printf '%s\n' "Building grove-operator with:
   GOOS: $GOOS
   GOARCH: $GOARCH
   ldflags: $ld_flags ..."

  CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} GO111MODULE=on \
    go build \
    -o "${BINARY_DIR}/grove-operator" \
    -ldflags "${ld_flags}" \
    cmd/main.go
}

mkdir -p ${BINARY_DIR}
build_grove_operator

