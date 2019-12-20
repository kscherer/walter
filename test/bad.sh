#!/bin/bash

trap "echo BADTEST && sleep 60" SIGINT SIGTERM

echo "START BAD"
stress --cpu 1 --timeout 3s
