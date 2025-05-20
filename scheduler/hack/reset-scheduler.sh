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

set -o nounset
set -o pipefail

CLUSTER_NAME="scheduler-test-cluster"

function check_prereq() {
  if ! command -v docker &>/dev/null; then
    echo >&2 "docker is not installed, please install the docker CLI"
    exit 1
  fi
}

function reset_scheduler() {
  # Remove the custom kube-scheduler configuration.
  docker exec $CLUSTER_NAME-control-plane rm /etc/kubernetes/scheduler-configuration.yaml
  # Restore to the previously stored default static pod manifest.
  docker exec $CLUSTER_NAME-control-plane mv /etc/kubernetes/kube-scheduler.yaml /etc/kubernetes/manifests/kube-scheduler.yaml > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo -e "\033[0;31mAn error occurred while restoring the default kube-scheduler static pod manifest.\033[0m"
    echo -e "\033[0;31mThis target can only be successfully run after replace-default-scheduler.\033[0m"
  fi
}

function main() {
  echo "Checking prerequisites..."
  check_prereq
  echo  "Resetting the kube-scheduler to the default..."
  reset_scheduler
}

main
