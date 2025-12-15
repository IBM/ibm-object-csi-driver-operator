#!/bin/bash
#******************************************************************************
# Copyright 2021 IBM Corp.
# Licensed under the Apache License, Version 2.0 (the "License");
# You may not use this file except in compliance with the License.
#******************************************************************************
#
# This script calculates the test coverage from cover.html and outputs the percentage.
#
# It is called by the GitHub Action in the pipeline to calculate the coverage percentage.

# Extract the coverage percentage from cover.html
COVERAGE=$(cat cover.html | grep "%)" | sed 's/[][()><%]/ /g' | awk '{ print $4 }' | awk '{s+=$1}END{print s/NR}')
echo "-------------------------------------------------------------------------"
echo "COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"
