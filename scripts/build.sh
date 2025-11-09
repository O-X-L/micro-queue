#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."
BASE_DIR="$(pwd)"

PATH_OUT="$(pwd)/build"
mkdir -p "$PATH_OUT"

go build -o "${PATH_OUT}/micro-queue" ./cmd/main.go

echo ''
echo '### DONE ###'
echo ''
ls "$PATH_OUT"
