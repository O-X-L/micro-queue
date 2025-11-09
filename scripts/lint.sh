#!/usr/bin/env bash

if [ -z "$CI" ]
then
  CI=0
fi

if [ -z "$1" ]
then
  set -euo pipefail
else
  set -uo pipefail
fi

cd "$(dirname "$0")/.."
BASE_DIR="$(pwd)"

function lint() {
  echo 'WARNINGS:'
  echo ''
  warnings="$(golangci-lint --config=./.golangci_warn.yml run || true)"
  if [[ "$CI" != "0" ]]
  then
    echo "$warnings"
  else
    # should be shown as warning in github-CI action-overview
    echo "::warning $warnings"
  fi
  echo ''
  echo 'ERRORS:'
  echo ''
  golangci-lint --config=./.golangci_fail.yml run
}

echo ''
echo '### LINTING ###'
echo ''
cd "${BASE_DIR}"
lint
