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
MODULE_ROOT="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(dirname "$MODULE_ROOT")"
REPO_HACK_DIR=${REPO_ROOT}/hack
TOOLS_BIN_DIR="${REPO_HACK_DIR}/tools/bin"

trap cleanup EXIT

function setup() {
  # kube_codegen.sh NEEDS to run from inside the Go module, as it uses go.mod for dependency versions.
  # Symbolic linking does not work either, since it uses `pwd -P` inside. Only copying will work.
  mkdir -p ${MODULE_ROOT}/hack/tools/bin/
  cp -f ${TOOLS_BIN_DIR}/kube_codegen.sh ${MODULE_ROOT}/hack/tools/bin/
  source "${MODULE_ROOT}/hack/tools/bin/kube_codegen.sh"
  # ensure that the version of code-generator used is the same as that of k8s.io/api
  k8s_api_version=$(go list -mod=mod -f '{{ .Version }}' -m k8s.io/api)
  go get -tool k8s.io/code-generator@${k8s_api_version}
}

function cleanup() {
  rm -rf ${MODULE_ROOT}/hack/tools
}

function generate_deepcopy_defaulter() {
 kube::codegen::gen_helpers \
    --boilerplate "${REPO_HACK_DIR}/boilerplate.go.txt" \
    "${MODULE_ROOT}/api"
}

function generate_clientset() {
  kube::codegen::gen_client \
    --with-watch \
    --output-dir "${MODULE_ROOT}/client" \
    --output-pkg "github.com/NVIDIA/grove/operator/client" \
    --boilerplate "${REPO_HACK_DIR}/boilerplate.go.txt" \
    "${MODULE_ROOT}/api"
}

function main() {
  setup

  echo "> Generate..."
  go generate "${MODULE_ROOT}/..."

  echo "> Generating DeepCopy and Defaulting functions..."
  generate_deepcopy_defaulter

  echo "> Generating ClientSet for PodGangSet API..."
  generate_clientset
}

main
