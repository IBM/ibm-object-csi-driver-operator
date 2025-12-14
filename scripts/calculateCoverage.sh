#!/bin/bash
#******************************************************************************
# Copyright 2021 IBM Corp.
# Licensed under the Apache License, Version 2.0
#******************************************************************************
set -euo pipefail

go tool cover -html=coverage.out -o cover.html

COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')

COVERAGE=${COVERAGE:-0.00}

echo "-------------------------------------------------------------------------"
echo "COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"