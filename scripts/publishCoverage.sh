#!/bin/bash
#******************************************************************************
# Copyright 2022 IBM Corp.
# Licensed under the Apache License, Version 2.0
#******************************************************************************
set -euo pipefail

echo "Publishing coverage results..."

if [ -z "${GHE_TOKEN:-}" ]; then
  echo "GHE_TOKEN not set → skipping coverage publish (normal in forks)"
  exit 0
fi

REPO="$GITHUB_REPOSITORY"
BRANCH="${GITHUB_REF_NAME:-master}"
COMMIT="$GITHUB_SHA"

# Use coverage from calculateCoverage.sh
if [ -f cover.html ]; then
  NEW_COVERAGE=$(grep -o '[0-9.]*%' cover.html | head -1 | tr -d '%')
else
  NEW_COVERAGE="0.00"
fi

NEW_COVERAGE=$(printf "%.2f" "${NEW_COVERAGE:-0.00}")

echo "Current coverage: ${NEW_COVERAGE}%"

# Badge color
COLOR="red"
if (( $(echo "$NEW_COVERAGE >= 85" | bc -l) )); then
  COLOR="brightgreen"
elif (( $(echo "$NEW_COVERAGE >= 70" | bc -l) )); then
  COLOR="green"
elif (( $(echo "$NEW_COVERAGE >= 50" | bc -l) )); then
  COLOR="yellow"
fi

# Temp dir for gh-pages
WORKDIR=$(mktemp -d)
cd "$WORKDIR"

if ! git clone -q -b gh-pages "https://x-access-token:$GHE_TOKEN@github.com/$REPO.git" . 2>/dev/null; then
  echo "Creating new gh-pages branch"
  git init -q
  git checkout -b gh-pages
fi

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

mkdir -p "coverage/$BRANCH" "coverage/$COMMIT"

cp "$GITHUB_WORKSPACE/cover.html" "coverage/$BRANCH/cover.html"
cp "$GITHUB_WORKSPACE/cover.html" "coverage/$COMMIT/cover.html"

curl -s "https://img.shields.io/badge/coverage-${NEW_COVERAGE}%25-${COLOR}.svg" \
     -o "coverage/$BRANCH/badge.svg"

git add .
git commit -m "Coverage update: $COMMIT" || echo "No changes to commit"
git push "https://x-access-token:$GHE_TOKEN@github.com/$REPO.git" gh-pages

echo "Coverage published!"
echo "Badge URL: https://$REPO.github.io/coverage/$BRANCH/badge.svg"

# Comment on PR
if [ "$GITHUB_EVENT_NAME" = "pull_request" ]; then
  PR_NUMBER=$(jq -r .number "$GITHUB_EVENT_PATH")
  curl -s -X POST \
    -H "Authorization: token $GHE_TOKEN" \
    -H "Content-Type: application/json" \
    "https://api.github.com/repos/$REPO/issues/$PR_NUMBER/comments" \
    -d "{\"body\": \"**Code Coverage:** ${NEW_COVERAGE}%  \\n![coverage](https://$REPO.github.io/coverage/$BRANCH/badge.svg)\"}"
fi