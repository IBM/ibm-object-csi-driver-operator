#!/bin/bash
#******************************************************************************
# Copyright 2021 IBM Corp.
# Licensed under the Apache License, Version 2.0
#******************************************************************************
set -euo pipefail

# Generate HTML if missing
if [ ! -f cover.html ]; then
  if [ -f cover.out ]; then
    go tool cover -html=cover.out -o cover.html
  else
    echo "No coverage data
    exit 1
  fi
fi

# Extract the **total** coverage line (last line with "total")
TOTAL_LINE=$(go tool cover -func=cover.out | grep "total:" | tail -1)
COVERAGE=$(echo "$TOTAL_LINE" | awk '{print $3}' | sed 's/%//')

# Fallback
COVERAGE=${COVERAGE:-0.0}

echo "-------------------------------------------------------------------------"
echo "REAL COVERAGE (from 'total:' line): ${COVERAGE}%"
echo "-------------------------------------------------------------------------"