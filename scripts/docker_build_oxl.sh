#!/usr/bin/env bash

set -euo pipefail

if [ -z "$1" ]
then
  echo "ERROR: Supply a version"
  exit 1
fi

VERSION="$1"

cd "$(dirname "$0")/../docker"

docker build -f Dockerfile -t "oxlorg/micro-queue:${VERSION}" --network=host --build-arg VERSION="$VERSION" --no-cache .
docker build -f Dockerfile -t "oxlorg/micro-queue:latest" --network=host --build-arg VERSION="$VERSION" .

docker push "oxlorg/micro-queue:${VERSION}"
docker push "oxlorg/micro-queue:latest"
