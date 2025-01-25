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
TOOLS_BIN_DIR="${SCRIPT_DIR}/tools/bin"

source "${TOOLS_BIN_DIR}/kube_codegen.sh"

function generate_deepcopy_defaulter() {
 kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_DIR}/boilerplate.go.txt" \
    "${OPERATOR_GO_MODULE_ROOT}/api"
}

function generate_clientset() {
  kube::codegen::gen_client \
    --with-watch \
    --output-dir "${OPERATOR_GO_MODULE_ROOT}/client" \
    --output-pkg "github.com/NVIDIA/grove/operator/client" \
    --boilerplate "${SCRIPT_DIR}/boilerplate.go.txt" \
    "${OPERATOR_GO_MODULE_ROOT}/api"
}

function main() {
  echo "> Generate..."
  go generate "${OPERATOR_GO_MODULE_ROOT}/..."

  echo "> Generating DeepCopy and Defaulting functions..."
  generate_deepcopy_defaulter

  echo "> Generating ClientSet for PodGangSet API..."
  generate_clientset
}

main
