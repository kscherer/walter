#!/bin/bash

trap "echo GOODTEST" SIGINT SIGTERM

echo "START GOOD"
stress --cpu 1 --timeout 3s
