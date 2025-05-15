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
REPO_ROOT="$(dirname $MODULE_ROOT)"
KIND_CONFIG_DIR="${SCRIPT_DIR}/kind"
IMAGE=""
GOARCH=${GOARCH:-$(go env GOARCH)}
PLATFORM=${PLATFORM:-linux/${GOARCH}}
CLUSTER_NAME="scheduler-test-cluster"
REG_PORT='5001'

PACKAGE_PATH=${PACKAGE_PATH}
PROGRAM_NAME=${PROGRAM_NAME}
VERSION="$(cat "${MODULE_ROOT}/VERSION")"

source $REPO_ROOT/hack/ld-flags.sh

function check_prereq() {
  if ! command -v skaffold &>/dev/null; then
    echo >&2 "skaffold is not installed, please install skaffold from https://skaffold.dev/docs/install/"
    exit 1
  fi
  if ! command -v yq &>/dev/null; then
    echo >&2 "yq is not installed, please install yq from https://github.com/mikefarah/yq/releases"
    exit 1
  fi
}

function skaffold_build() {
  local version="$(cat "${MODULE_ROOT}/VERSION")"
  export VERSION="$version"
  local ld_flags=$(build_ld_flags)
  export LD_FLAGS="${ld_flags}"
  # default registry is needed if there is no current kube context
  IMAGE=$(skaffold build --platform=$PLATFORM --quiet --output="{{range .Builds}}{{.Tag}}{{println}}{{end}}" --default-repo localhost:$REG_PORT --module scheduler)
}

function copy_kube_scheduler_configuration() {
  docker cp ${KIND_CONFIG_DIR}/scheduler-configuration.yaml $CLUSTER_NAME-control-plane:/etc/kubernetes/scheduler-configuration.yaml
  docker exec $CLUSTER_NAME-control-plane chmod a+r /etc/kubernetes/scheduler-configuration.yaml
}

function replace_kube_scheduler_manifest() {
  docker exec $CLUSTER_NAME-control-plane chmod a+r /etc/kubernetes/scheduler.conf
  yq '.spec.containers[0].image = "'"$IMAGE"'"' ${KIND_CONFIG_DIR}/kube-scheduler.yaml | \
    docker exec -i $CLUSTER_NAME-control-plane tee /etc/kubernetes/manifests/kube-scheduler.yaml > /dev/null
}

# The script exists in this form because `skaffold dev` `skaffold run`, etc can not skip the deployment of the image.
# Post/Pre deployment hooks don't work. Not using build hooks since building the image will conflate with the deployment.
# Issue for custom deployment is still open https://github.com/GoogleContainerTools/skaffold/issues/2277.
function main() {
  echo "Checking prerequisites..."
  check_prereq
  echo "Building kube-scheduler..."
  skaffold_build
  echo "Copying kube-scheduler configuration..."
  copy_kube_scheduler_configuration
  echo "Replacing the kube-scheduler static pod manifest..."
  replace_kube_scheduler_manifest
}

main
