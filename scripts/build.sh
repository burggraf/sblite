#!/bin/bash
set -e

# Local build script for sblite
# Reads version from VERSION file

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Read version from VERSION file
VERSION_FILE="${PROJECT_ROOT}/VERSION"
if [ ! -f "$VERSION_FILE" ]; then
    echo "Error: VERSION file not found"
    exit 1
fi

VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')

# Build info
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS="-s -w -X github.com/markb/sblite/cmd.Version=${VERSION} -X github.com/markb/sblite/cmd.BuildTime=${BUILD_TIME} -X github.com/markb/sblite/cmd.GitCommit=${GIT_COMMIT}"

echo "Building sblite v${VERSION}..."
go build -ldflags "${LDFLAGS}" -o "${PROJECT_ROOT}/sblite" "${PROJECT_ROOT}"

echo "Done: ./sblite v${VERSION}"
