#!/bin/bash

./bin/walter --config test/pipeline.yml --stage "$1" &

KILL_IN_SEC=${2:-2}
PID=$!

sleep "$KILL_IN_SEC"

kill -SIGINT $PID
