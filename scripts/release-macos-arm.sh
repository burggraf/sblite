#!/bin/bash
set -e

# Release script for sblite macOS ARM64
# Usage: ./scripts/release-macos-arm.sh [version]
# If version not provided, reads from VERSION file

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Get version from argument or VERSION file
if [ -n "$1" ]; then
    VERSION="$1"
else
    VERSION_FILE="${PROJECT_ROOT}/VERSION"
    if [ ! -f "$VERSION_FILE" ]; then
        echo "Error: VERSION file not found and no version argument provided"
        exit 1
    fi
    VERSION="v$(cat "$VERSION_FILE" | tr -d '[:space:]')"
fi

# Validate version format (should start with 'v')
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version should be in format vX.Y.Z (e.g., v1.0.0)"
    exit 1
fi

# Configuration
BINARY_NAME="sblite"
GOOS="darwin"
GOARCH="arm64"
BUILD_DIR="${PROJECT_ROOT}/dist"
ARCHIVE_NAME="${BINARY_NAME}-${VERSION}-${GOOS}-${GOARCH}.zip"

echo "==> Building ${BINARY_NAME} ${VERSION} for ${GOOS}/${GOARCH}..."

# Clean and create build directory
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

# Build with version info embedded
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS="-s -w -X github.com/markb/sblite/cmd.Version=${VERSION#v} -X github.com/markb/sblite/cmd.BuildTime=${BUILD_TIME} -X github.com/markb/sblite/cmd.GitCommit=${GIT_COMMIT}"

GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags "${LDFLAGS}" -o "${BUILD_DIR}/${BINARY_NAME}" "${PROJECT_ROOT}"

echo "==> Binary built successfully"

# Verify the binary
echo "==> Verifying binary..."
"${BUILD_DIR}/${BINARY_NAME}" --version

# Create zip archive
echo "==> Creating archive ${ARCHIVE_NAME}..."
cd "${BUILD_DIR}"
zip "${ARCHIVE_NAME}" "${BINARY_NAME}"
cd ..

echo "==> Archive created: ${BUILD_DIR}/${ARCHIVE_NAME}"

# Check if gh CLI is available
if ! command -v gh &> /dev/null; then
    echo "Error: GitHub CLI (gh) is not installed."
    echo "Install it with: brew install gh"
    echo ""
    echo "Archive is ready at: ${BUILD_DIR}/${ARCHIVE_NAME}"
    echo "You can manually upload it to GitHub releases."
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "Error: Not authenticated with GitHub CLI."
    echo "Run: gh auth login"
    exit 1
fi

# Create release and upload
echo "==> Creating GitHub release ${VERSION}..."

# Check if release already exists
if gh release view "${VERSION}" &> /dev/null; then
    echo "Release ${VERSION} already exists. Uploading asset to existing release..."
    gh release upload "${VERSION}" "${BUILD_DIR}/${ARCHIVE_NAME}" --clobber
else
    echo "Creating new release ${VERSION}..."
    gh release create "${VERSION}" \
        "${BUILD_DIR}/${ARCHIVE_NAME}" \
        --title "sblite ${VERSION}" \
        --generate-notes
fi

echo ""
echo "==> Release ${VERSION} published successfully!"
echo "View at: https://github.com/burggraf/sblite/releases/tag/${VERSION}"
