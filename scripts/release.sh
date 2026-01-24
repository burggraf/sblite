#!/bin/bash
set -e

# Release script for sblite
# Creates and pushes a git tag based on VERSION file

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Read version from VERSION file
VERSION_FILE="${PROJECT_ROOT}/VERSION"
if [ ! -f "$VERSION_FILE" ]; then
    echo "Error: VERSION file not found"
    exit 1
fi

VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')
TAG="v${VERSION}"

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "Error: Tag ${TAG} already exists"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "Error: You have uncommitted changes. Please commit or stash them first."
    exit 1
fi

echo "Creating release ${TAG}..."

# Create and push tag
git tag "$TAG"
echo "Created tag ${TAG}"

git push origin "$TAG"
echo "Pushed tag ${TAG} to origin"

echo "Release ${TAG} complete!"
