#!/usr/bin/env bash

HEAD_REF=${GITHUB_HEAD_REF:-${GITHUB_REF_NAME}}
BASE_REF="master"

EXECUTE="true"

make tidy

if [[ $(git ls-files --modified | wc -l | xargs) -gt 0 ]]; then
    EXECUTE="false"
fi

echo "passed=$EXECUTE" >> "$GITHUB_OUTPUT"
