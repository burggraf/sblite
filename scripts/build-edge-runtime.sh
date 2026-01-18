#!/bin/bash
#
# Build edge-runtime for local development
# This script compiles the Supabase edge-runtime from source for macOS ARM
#

set -e

# Configuration
EDGE_RUNTIME_VERSION="v1.67.4"
EDGE_RUNTIME_REPO="https://github.com/supabase/edge-runtime.git"
BUILD_DIR="${TMPDIR:-/tmp}/edge-runtime-build"
INSTALL_DIR="${HOME}/.local/share/sblite/bin"
BINARY_NAME="edge-runtime-${EDGE_RUNTIME_VERSION}"

echo "=== Edge Runtime Build Script ==="
echo "Version: ${EDGE_RUNTIME_VERSION}"
echo "Install dir: ${INSTALL_DIR}"
echo ""

# Check if already installed
if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    echo "Edge runtime already installed at ${INSTALL_DIR}/${BINARY_NAME}"
    echo "Remove it first if you want to rebuild:"
    echo "  rm ${INSTALL_DIR}/${BINARY_NAME}"
    exit 0
fi

# Check for Rust
if ! command -v cargo &> /dev/null; then
    echo "Rust/Cargo not found. Installing..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    source "${HOME}/.cargo/env"
fi

echo "Using Rust: $(rustc --version)"
echo "Using Cargo: $(cargo --version)"
echo ""

# Check for OpenBLAS on macOS
if [[ "$OSTYPE" == "darwin"* ]]; then
    if [ -d "/opt/homebrew/opt/openblas" ]; then
        export LDFLAGS="-L/opt/homebrew/opt/openblas/lib"
        export CPPFLAGS="-I/opt/homebrew/opt/openblas/include"
        export PKG_CONFIG_PATH="/opt/homebrew/opt/openblas/lib/pkgconfig"
        echo "Using OpenBLAS from Homebrew"
    elif [ -d "/usr/local/opt/openblas" ]; then
        export LDFLAGS="-L/usr/local/opt/openblas/lib"
        export CPPFLAGS="-I/usr/local/opt/openblas/include"
        export PKG_CONFIG_PATH="/usr/local/opt/openblas/lib/pkgconfig"
        echo "Using OpenBLAS from Homebrew (Intel)"
    else
        echo "WARNING: OpenBLAS not found. Install with: brew install openblas"
    fi
fi

# Clean up any previous build
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

# Clone the repository
echo "=== Cloning edge-runtime ${EDGE_RUNTIME_VERSION} ==="
cd "${BUILD_DIR}"
git clone --depth 1 --branch "${EDGE_RUNTIME_VERSION}" "${EDGE_RUNTIME_REPO}" .

# Build
echo ""
echo "=== Building edge-runtime (this may take 10-20 minutes) ==="
echo "Note: Requires significant RAM. If build fails, try closing other apps."
echo ""

# Set version tag for build
export GIT_V_TAG="${EDGE_RUNTIME_VERSION#v}"

# Build release binary
cargo build --release

# Check if build succeeded
if [ ! -f "target/release/edge-runtime" ]; then
    echo "ERROR: Build failed - binary not found"
    exit 1
fi

# Install
echo ""
echo "=== Installing edge-runtime ==="
mkdir -p "${INSTALL_DIR}"
cp "target/release/edge-runtime" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Verify
echo ""
echo "=== Verifying installation ==="
"${INSTALL_DIR}/${BINARY_NAME}" --version || echo "(version check may fail, that's ok)"

# Clean up
echo ""
echo "=== Cleaning up build directory ==="
rm -rf "${BUILD_DIR}"

echo ""
echo "=== SUCCESS ==="
echo "Edge runtime installed to: ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "You can now run: ./sblite serve --functions"
