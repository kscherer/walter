#!/bin/bash

trap "rm -f test/temp/TaskAbort1.tmp" SIGINT SIGTERM

echo "Abort test ..."
stress --cpu 1 --timeout 30s
