#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."

function kill_mq() {
  echo 'killing'
  pkill -f build/micro-queue
  sleep 1
}

function failure() {
  echo ''
  echo '### FAILED ###'
  echo ''
  kill_mq
  exit 1
}

function success() {
  echo ''
  echo '### SUCCESS ###'
  echo ''
  kill_mq
  exit 0
}

# kill_mq

echo '### STARTING ###'
build/micro-queue -c $(pwd)/testdata/config.yml &
sleep 1

echo '### POSTING TO QUEUE ###'
if ! curl -v --fail -X POST -H "Authorization: Bearer aa9d7b0a-8bee-47e4-887c-bdb799533793" http://127.0.0.1:10000/in/fetcher --data '{"context": "test"}'
then
  failure
fi

echo '### READING FROM QUEUE ###'
if ! curl -v --fail -X GET -H "Authorization: Bearer bb9d7b0a-8bee-47e4-887c-bdb799533773" http://127.0.0.1:10000/out/fetcher
then
  failure
fi

success
