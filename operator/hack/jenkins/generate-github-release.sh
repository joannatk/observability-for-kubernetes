#!/usr/bin/env bash
set -e

OPERATOR_REPO_ROOT=$(git rev-parse --show-toplevel)/operator

operator_yaml="${OPERATOR_REPO_ROOT}/deploy/kubernetes/wavefront-operator.yaml"

VERSION=$(cat ${OPERATOR_REPO_ROOT}/release/OPERATOR_VERSION)
GITHUB_REPO=wavefrontHQ/observability-for-kubernetes
AUTH="Authorization: token ${GITHUB_TOKEN}"

id=$(curl --fail -X POST -H "Content-Type:application/json" \
-H "$AUTH" \
-d "{
      \"tag_name\": \"v$VERSION\",
      \"target_commitish\": \"$GIT_BRANCH\",
      \"name\": \"Release v$VERSION\",
      \"body\": \"Description for v$VERSION\",
      \"draft\": true,
      \"prerelease\": false}" \
"https://api.github.com/repos/$GITHUB_REPO/releases" | jq ".id")

curl --data-binary @"$operator_yaml" \
  -H "$AUTH" \
  -H "Content-Type: application/octet-stream" \
"https://uploads.github.com/repos/$GITHUB_REPO/releases/$id/assets?name=$(basename $operator_yaml)"
