#!/usr/bin/env bash
set -e

git fetch --tags

version=$(git tag --sort=-version:refname | head -1)
version=${version:-0.0.0}

major=${version%%.*}
version=${version#*.}
minor=${version%%.*}
version=${version#*.}
patch=${version%%.*}

patch=$((patch + 1))
new_tag="$major.$minor.$patch-beta"

echo "Updating to $new_tag"
git tag -s -m "$new_tag" "$new_tag"