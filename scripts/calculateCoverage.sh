#!/bin/bash
#******************************************************************************
# Copyright 2021 IBM Corp.
# Licensed under the Apache License, Version 2.0
#******************************************************************************
set -euo pipefail

# Operator Makefile generates coverage.out → filtered → cover.out → cover.html
# We ensure cover.html exists and extract the total coverage %

if [ ! -f cover.html ]; then
  if [ -f cover.out ]; then
    echo "Generating cover.html from cover.out..."
    go tool cover -html=cover.out -o cover.html
  else
    echo "ERROR: No coverage data found (missing coverage.out)"
    exit 1
  fi
fi

# Extract total coverage % from cover.html (exact same logic as COS CSI driver)
COVERAGE=$(grep -o '[0-9.]*%' cover.html | head -1 | tr -d '%')

# Fallback: use go tool if HTML parsing fails
if [ -z "$COVERAGE" ] || [ "$COVERAGE" = "" ]; then
  COVERAGE=$(go tool cover -func=coverage.out 2>/dev/null | grep total | awk '{print $3}' | tr -d '%')
fi

COVERAGE=${COVERAGE:-0.00}

echo "-------------------------------------------------------------------------"
echo "COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"