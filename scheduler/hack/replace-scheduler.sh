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
KIND_CONFIG_DIR="${SCRIPT_DIR}/kind"
IMAGE=""
GOARCH=${GOARCH:-$(go env GOARCH)}
PLATFORM=${PLATFORM:-linux/${GOARCH}}
CLUSTER_NAME="scheduler-test-cluster"
REG_PORT='5001'

source $(dirname $0)/ld-flags.sh

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
  IMAGE=$(skaffold build --platform=linux/$GOARCH --quiet --output="{{range .Builds}}{{.Tag}}{{println}}{{end}}" --default-repo localhost:$REG_PORT --module scheduler-plugins-replace)
}

function generate_kube_scheduler_configuration() {
  cat >"${KIND_CONFIG_DIR}/scheduler-configuration.yaml" <<EOF
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
leaderElection:
  # (Optional) Change true to false if you are not running a HA control-plane.
  leaderElect: false
clientConnection:
  kubeconfig: /etc/kubernetes/scheduler.conf
EOF
}

function generate_kube_scheduler_manifest() {
  cat >"${KIND_CONFIG_DIR}/kube-scheduler.yaml" <<EOF
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  labels:
    component: kube-scheduler
    tier: control-plane
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
  - command:
    - kube-scheduler
    - --authentication-kubeconfig=/etc/kubernetes/scheduler.conf
    - --authorization-kubeconfig=/etc/kubernetes/scheduler.conf
    - --bind-address=127.0.0.1
    - --config=/etc/kubernetes/scheduler-configuration.yaml
    image: image_template
    imagePullPolicy: Always
    livenessProbe:
      failureThreshold: 8
      httpGet:
        host: 127.0.0.1
        path: /livez
        port: 10259
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    name: kube-scheduler
    readinessProbe:
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /readyz
        port: 10259
        scheme: HTTPS
      periodSeconds: 1
      timeoutSeconds: 15
    resources:
      requests:
        cpu: 100m
    startupProbe:
      failureThreshold: 24
      httpGet:
        host: 127.0.0.1
        path: /livez
        port: 10259
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /etc/kubernetes/scheduler.conf
      name: kubeconfig
      readOnly: true
    - mountPath: /etc/kubernetes/scheduler-configuration.yaml
      name: scheduler-configuration
      readOnly: true
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
  - hostPath:
      path: /etc/kubernetes/scheduler.conf
      type: FileOrCreate
    name: kubeconfig
  - hostPath:
      path: /etc/kubernetes/scheduler-configuration.yaml
      type: FileOrCreate
    name: scheduler-configuration
status: {}
EOF

  yq -i '.spec.containers[0].image = "'"$IMAGE"'"' ${KIND_CONFIG_DIR}/kube-scheduler.yaml
}

function copy_kube_scheduler_configuration() {
  docker cp ${KIND_CONFIG_DIR}/scheduler-configuration.yaml $CLUSTER_NAME-control-plane:/etc/kubernetes/scheduler-configuration.yaml
  docker exec $CLUSTER_NAME-control-plane chmod a+r /etc/kubernetes/scheduler-configuration.yaml
}

function replace_kube_scheduler_manifest() {
  docker exec $CLUSTER_NAME-control-plane chmod a+r /etc/kubernetes/scheduler.conf
  docker cp ${KIND_CONFIG_DIR}/kube-scheduler.yaml $CLUSTER_NAME-control-plane:/etc/kubernetes/manifests/kube-scheduler.yaml
}

# The script exists in this form because `skaffold dev` `skaffold run`, etc can not skip the deployment of the image.
# Post/Pre deployment hooks don't work. Not using build hooks since building the image will conflate with the deployment.
# Issue for custom deployment is still open https://github.com/GoogleContainerTools/skaffold/issues/2277.
function main() {
  echo "Checking prerequisites..."
  check_prereq
  echo "Building kube-scheduler..."
  skaffold_build
  echo "Generating kube-scheduler configruation..."
  generate_kube_scheduler_configuration
  echo "Generating kube-scheduler static pod manifest..."
  generate_kube_scheduler_manifest
  echo "Copying kube-scheduler configuration..."
  copy_kube_scheduler_configuration
  echo "Replacing the kube-scheduler static pod manifest..."
  replace_kube_scheduler_manifest
}

main
