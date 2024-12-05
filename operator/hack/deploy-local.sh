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


function check_prereq() {
  if ! command -v skaffold &>/dev/null; then
    echo >&2 "skaffold is not available, please install skaffold from https://skaffold.dev/docs/install/"
    exit 1
  fi
}

function copy_crds() {
  declare -a crds=("grove.io_podgangs.yaml" "grove.io_podgangsets.yaml")
  targetPath="${OPERATOR_GO_MODULE_ROOT}/charts/crds"
  for crd in "${crds[@]}"; do
    local crdPath="${OPERATOR_GO_MODULE_ROOT}/config/crd/bases/${crd}"
    if [ ! -f ${crdPath} ]; then
      echo >&2 "CRD ${crd} not found in ${OPERATOR_GO_MODULE_ROOT}/config/crd/bases, run 'make generate' first"
      exit 1
    fi
    echo "Copying CRD ${crd} to ${targetPath}"
    cp ${crdPath} ${targetPath}
  done
}

function skaffold_deploy() {
  local ld_flags=$(build_ld_flags)
  export LD_FLAGS="${ld_flags}"
  skaffold "$@"
}

check_prereq
copy_crds
skaffold_deploy "$@"

