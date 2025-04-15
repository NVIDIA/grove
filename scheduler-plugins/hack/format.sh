#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "> Format"

for p in "$@" ; do
  goimports-reviser -rm-unused \
   -imports-order "std,company,project,general,blanked,dotted" \
   -format \
   -recursive $p
done
