#!/usr/bin/env bash

set -euo pipefail

if [ -z "$1" ]
then
  echo "ERROR: Supply a version"
  exit 1
fi

VERSION="$1"

cd "$(dirname "$0")/../docker"

docker build -f Dockerfile -t queue-local --network=host --build-arg VERSION="$VERSION" --no-cache .
