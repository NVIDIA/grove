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

source $(dirname $0)/openssl-utils.sh

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
OPERATOR_GO_MODULE_ROOT="$(dirname "$SCRIPT_DIR")"
CHARTS_DIR="${OPERATOR_GO_MODULE_ROOT}/charts"

function check_prereq() {
  if ! command -v kubectl &>/dev/null; then
    echo >&2 "kubectl is not installed, please install kubectl from https://kubernetes.io/docs/tasks/tools/install-kubectl/"
    exit 1
  fi
}

function copy_crds() {
  declare -a crds=("grove.io_podgangsets.yaml" "grove.io_podcliques.yaml")
  target_path="${OPERATOR_GO_MODULE_ROOT}/charts/crds"
  echo "Creating ${target_path} to copy the CRDs if not present..."
  mkdir -p ${target_path}
  for crd in "${crds[@]}"; do
    local crd_path="${OPERATOR_GO_MODULE_ROOT}/api/core/crds/${crd}"
    if [ ! -f ${crd_path} ]; then
      echo "CRD ${crd} not found in ${OPERATOR_GO_MODULE_ROOT}/api/core/crds, running 'make generate' first"
      make generate
    fi
    echo "Copying CRD ${crd} to ${target_path}"
    cp ${crd_path} ${target_path}
  done
}

function initialize_pki_resources() {
  if [[ $# -ne 2 ]]; then
    echo -e "${FUNCNAME[0]} requires 2 arguments: namespace and cert-expiry"
    exit 1
  fi

  local namespace="$1"
  local cert_expiry="$2"
  local generation_required=false

  target_path="${OPERATOR_GO_MODULE_ROOT}/charts/pki-resources"
  if [ ! -d ${target_path} ]; then
    mkdir -p ${target_path}
    generation_required=true
  fi

  if ${generation_required} || ! all_pki_resources_exist; then
    echo "Generating PKI resources..."
    rm -rf ${target_path}/*
    pki::generate_resources "${target_path}" "${namespace}"
  fi
}

function all_pki_resources_exist() {
  sourceDir="${OPERATOR_GO_MODULE_ROOT}/charts/pki-resources/"
  PKI_RESOURCES=(
   "ca.crt"
   "ca.key"
   "server.crt"
   "server.key"
  )
  for resource in "${PKI_RESOURCES[@]}"; do
    local resourcePath="${sourceDir}/${resource}"
    if [ ! -f ${resourcePath} ]; then
      return 1
    fi
  done
  return 0
}

function prepare_local_deploy() {
  if [[ $# -ne 2 ]]; then
    echo -e "${FUNCNAME[0]} requires 2 arguments: namespace and cert-expiry"
    exit 1
  fi
  echo "Copying CRDs to helm charts..."
  copy_crds
  echo "Generating PKI resources if not present or expired..."
  local namespace="$1"
  local cert_expiry="$2"
  if [[ "${namespace}" != "default" ]]; then
    found=$(kubectl get ns "${namespace}" --ignore-not-found)
    if [[ -z "${found}" ]]; then
      kubectl create namespace "${namespace}"
    fi
  fi
  initialize_pki_resources "$@"
}

check_prereq
prepare_local_deploy "$@"