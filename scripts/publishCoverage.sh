#!/bin/bash
#******************************************************************************
# Copyright 2022 IBM Corp.
# Licensed under the Apache License, Version 2.0
#******************************************************************************
set -euo pipefail

echo "Publishing coverage..."

if [ -z "${GHE_TOKEN:-}" ]; then
  echo "No GHE_TOKEN → skipping publish (normal in forks)"
  exit 0
fi

REPO="$GITHUB_REPOSITORY"
BRANCH="${GITHUB_REF_NAME:-main}"

COVERAGE=$(grep -o '[0-9.]*%' cover.html | head -1 | tr -d '%')
COVERAGE=$(printf "%.2f" "${COVERAGE:-0.00}")

echo "Coverage: ${COVERAGE}%"

COLOR="red"
if (( $(echo "$COVERAGE >= 85" | bc -l) )); then
  COLOR="brightgreen"
elif (( $(echo "$COVERAGE >= 70" | bc -l) )); then
  COLOR="green"
elif (( $(echo "$COVERAGE >= 50" | bc -l) )); then
  COLOR="yellow"
fi

WORKDIR=$(mktemp -d)
cd "$WORKDIR"

git clone -q -b gh-pages "https://x-access-token:$GHE_TOKEN@github.com/$REPO.git" . 2>/dev/null || \
  (git init -q && git checkout -b gh-pages)

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

mkdir -p "coverage/$BRANCH"
cp "$GITHUB_WORKSPACE/cover.html" "coverage/$BRANCH/cover.html"

curl -s "https://img.shields.io/badge/coverage-${COVERAGE}%25-${COLOR}.svg" -o "coverage/$BRANCH/badge.svg"

git add .
git commit -m "Coverage update: $GITHUB_SHA" || echo "Nothing to commit"
git push "https://x-access-token:$GHE_TOKEN@github.com/$REPO.git" gh-pages

echo "Published to https://$REPO.github.io/coverage/$BRANCH/badge.svg"

if [ "$GITHUB_EVENT_NAME" = "pull_request" ]; then
  PR_NUM=$(jq -r .number "$GITHUB_EVENT_PATH")
  curl -s -X POST \
    -H "Authorization: token $GHE_TOKEN" \
    -d "{\"body\": \"**Code Coverage:** ${COVERAGE}%  \\n![badge](https://$REPO.github.io/coverage/$BRANCH/badge.svg)\"}" \
    "https://api.github.com/repos/$REPO/issues/$PR_NUM/comments"
fi