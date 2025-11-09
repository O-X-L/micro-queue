#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."

BASE_DIR="$(pwd)"

echo ''
echo '### UNIT TEST ###'
echo ''
cd "${BASE_DIR}"
go run gotest.tools/gotestsum@latest --format pkgname ./...
