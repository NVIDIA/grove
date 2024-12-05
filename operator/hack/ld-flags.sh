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

# NOTE: This script should be sourced into other scripts.

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
OPERATOR_GO_MODULE_ROOT="$(dirname "$SCRIPT_DIR")"

# A reusable function which computes the LD Flags for the operator.
function build_ld_flags() {
  local package_path="github.com/NVIDIA/grove/operator/internal"
  local version="$(cat "${OPERATOR_GO_MODULE_ROOT}/VERSION")"
  local program_name="grove-operator"
  local build_date="$(date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$/:\1/g')"

  # .dockerignore ignores files that are not relevant for the build. It only copies the relevant source files to the
  # build container. Git, on the other hand will always detect a dirty work tree when building in a container (many deleted files).
  # This command filters out all deleted files that are ignored by .dockerignore and only detects changes to files that are included
  # to determine dirty work tree.
  local tree_state="$([ -z "$(git status --porcelain 2>/dev/null | grep -vf <(git ls-files -o --deleted --ignored --exclude-from=${OPERATOR_GO_MODULE_ROOT}/.dockerignore))" ] && echo clean || echo dirty)"

  echo "-X $package_path/version.gitVersion=$version
        -X $package_path/version.gitCommit=$(git rev-parse --verify HEAD)
        -X $package_path/version.gitTreeState=$tree_state
        -X $package_path/version.buildDate=$build_date
        -X $package_path/version.programName=$program_name"
}