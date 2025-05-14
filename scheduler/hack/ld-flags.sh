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

# NOTE: This script should be sourced into other scripts.

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
MODULE_ROOT="$(dirname "$SCRIPT_DIR")"

# A reusable function which computes the LD Flags for the kube-scheduler.
function build_ld_flags() {
  local package_path="k8s.io/component-base"
  local version="$(cat "${MODULE_ROOT}/VERSION")"
  # NOTE: the program_name override does not seem to be working, please troubelshoot later
  local program_name="kube-scheduler"
  local build_date="$(date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$/:\1/g')"

  # .dockerignore ignores files that are not relevant for the build. It only copies the relevant source files to the
  # build container. Git, on the other hand will always detect a dirty work tree when building in a container (many deleted files).
  # This command filters out all deleted files that are ignored by .dockerignore and only detects changes to files that are included
  # to determine dirty work tree.
  # Use `git status --porcelain $MODULE_ROOT` to only check the status of the directory
  local tree_state="$([ -z "$(git status --porcelain $MODULE_ROOT 2>/dev/null | grep -vf <(git ls-files -o --deleted --ignored --exclude-from=${MODULE_ROOT}/Dockerfile.dockerignore))" ] && echo clean || echo dirty)"

  # The k8s.component-base/version.gitCommit can not be set to the version of grove
  # due to the error: "emulation version 1.33 is not between [1.31, 0.1.0-dev]".
  # https://github.com/kubernetes/kubernetes/blob/5dc8b8dd268f2170286a75c142781f4db1da9020/staging/src/k8s.io/component-base/compatibility/version.go#L165
  echo "-X $package_path/version.gitCommit=$(git rev-parse --verify HEAD)
        -X $package_path/version.gitTreeState=$tree_state
        -X $package_path/version.buildDate=$build_date
        -X $package_path/version.programName=$program_name"
}