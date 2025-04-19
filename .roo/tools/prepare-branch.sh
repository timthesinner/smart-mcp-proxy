#!/bin/bash

# Usage: ./prepare-branch.sh "initial commit message" feature/branch

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 "initial commit message" feature/branch"
  exit 1
fi

initial_commit_message=$1
feature_branch=$2

set -e

# Reset to origin/main hard
git reset --hard origin/main

# Create an empty commit with the initial commit message
git commit --allow-empty -a -m "$initial_commit_message"

# Push the current HEAD to the remote feature branch
git push origin HEAD:$feature_branch

# Reset again to origin/main hard
git reset --hard origin/main

# Checkout the feature branch tracking the remote branch
git checkout -b $feature_branch --track origin/$feature_branch