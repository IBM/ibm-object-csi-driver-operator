#!/bin/bash
#******************************************************************************
# Copyright 2021 IBM Corp.
# Licensed under the Apache License, Version 2.0
#******************************************************************************
set -euo pipefail

if [ ! -f cover.html ]; then
  if [ -f cover.out ]; then
    echo "Generating cover.html from cover.out..."
    go tool cover -html=cover.out -o cover.html
  else
    echo "ERROR: No coverage data found"
    exit 1
  fi
fi

TOTAL_COVERAGE=$(go tool cover -func=cover.out | grep "total:" | tail -1 | awk '{print $3}' | sed 's/%//')
TOTAL_COVERAGE=${TOTAL_COVERAGE:-0.0}

echo "-------------------------------------------------------------------------"
echo "REAL COVERAGE: ${TOTAL_COVERAGE}%"
echo "-------------------------------------------------------------------------"
