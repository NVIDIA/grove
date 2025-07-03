#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
REPO_ROOT="$(dirname $SCRIPT_DIR)"

mkdir -p "${REPO_ROOT}/docs/api-reference"

echo "> Generating API docs for Grove Operator APIs..."
crd-ref-docs \
  --source-path "${REPO_ROOT}/operator/api" \
  --config "${SCRIPT_DIR}/apidocs/config.yaml" \
  --output-path "${REPO_ROOT}/docs/api-reference/operator-api.md" \
  --renderer markdown

echo "> Generating API docs for Grove Scheduler APIs..."
crd-ref-docs \
  --source-path "${REPO_ROOT}/scheduler/api/core" \
  --config "${SCRIPT_DIR}/apidocs/config.yaml" \
  --output-path "${REPO_ROOT}/docs/api-reference/scheduler-api.md" \
  --renderer markdown
