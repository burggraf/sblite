# Release Guide

This document describes how to create releases for sblite and the edge runtime.

## Overview

sblite uses GitHub Actions for automated builds:

- **sblite releases** - Triggered automatically on version tags (`v*`)
- **edge-runtime releases** - Triggered manually when updating the runtime version

### Supported Platforms

| Platform | sblite | Edge Runtime |
|----------|--------|--------------|
| macOS Apple Silicon (darwin-arm64) | Yes | Yes |
| macOS Intel (darwin-amd64) | Yes | Yes |
| Linux x86_64 (linux-amd64) | Yes | Yes |
| Linux ARM64 (linux-arm64) | Yes | Yes |
| Windows x86_64 (windows-amd64) | Yes | Docker only |
| Windows ARM64 (windows-arm64) | Yes | Docker only |

## Creating an sblite Release

### 1. Update Version

Update the version in relevant files if needed (e.g., `CLAUDE.md`, dashboard version display).

### 2. Create and Push Tag

```bash
git tag v0.2.7
git push origin v0.2.7
```

### 3. Wait for Build

The GitHub Actions workflow will automatically:
1. Build binaries for all 6 platforms
2. Create ZIP archives for each platform
3. Generate SHA256 checksums
4. Create a GitHub release with all assets

### 4. Verify Release

Check the [Releases page](https://github.com/burggraf/sblite/releases) to verify:
- All 6 ZIP files are present
- `checksums.txt` is included
- Release notes are generated

## Building Edge Runtime

The edge runtime is built from [supabase/edge-runtime](https://github.com/supabase/edge-runtime) source. This is a manual process since the runtime version changes infrequently.

### 1. Trigger the Build Workflow

1. Go to **Actions** → **Build Edge Runtime**
2. Click **Run workflow**
3. Enter the version (e.g., `v1.67.4`)
4. Click **Run workflow**

### 2. Wait for Builds

The workflow builds on native runners for each platform:
- `ubuntu-latest` → linux-amd64
- `ubuntu-24.04-arm64` → linux-arm64
- `macos-latest` → darwin-arm64
- `macos-13` → darwin-amd64

Build time is approximately 20-30 minutes (Rust compilation).

### 3. Verify Release

Check the releases page for `edge-runtime-v1.67.4` containing:
- 4 binary files (one per platform)
- `checksums.txt`

### 4. Update sblite Checksums

After the edge runtime release is created, update sblite to use the new checksums:

1. Download `checksums.txt` from the release
2. Edit `internal/functions/download.go`
3. Update the `edgeRuntimeChecksums` map:

```go
var edgeRuntimeChecksums = map[string]map[string]string{
    "v1.67.4": {
        "darwin-amd64": "<checksum from checksums.txt>",
        "darwin-arm64": "<checksum from checksums.txt>",
        "linux-amd64":  "<checksum from checksums.txt>",
        "linux-arm64":  "<checksum from checksums.txt>",
    },
}
```

4. Update `EdgeRuntimeVersion` constant if changing versions:

```go
const EdgeRuntimeVersion = "v1.67.4"
```

5. Commit and create a new sblite release

## Upgrading Edge Runtime Version

To upgrade to a new edge runtime version:

1. Check [supabase/edge-runtime releases](https://github.com/supabase/edge-runtime/releases) for the latest version
2. Run the edge-runtime build workflow with the new version
3. Wait for all 4 platform builds to complete
4. Update `internal/functions/download.go`:
   - Change `EdgeRuntimeVersion` constant
   - Add new version entry to `edgeRuntimeChecksums`
5. Test locally on your platform
6. Create a new sblite release

## Dashboard Runtime Installer

When users access the Edge Functions page in the dashboard and the runtime is not installed:

### Supported Platforms (macOS, Linux)

The dashboard shows a "Download & Install Runtime" button that:
1. Downloads the binary from GitHub releases
2. Shows progress bar during download
3. Verifies SHA256 checksum
4. Installs to `<db-dir>/edge-runtime/`

### Windows

Windows users see Docker instructions instead:

```bash
docker run -d -p 9000:9000 \
  -v ./functions:/functions \
  ghcr.io/supabase/edge-runtime:v1.67.4 \
  start --main-service /functions
```

## Release Artifacts

### sblite Release (e.g., v0.2.7)

```
sblite-v0.2.7-darwin-arm64.zip
sblite-v0.2.7-darwin-amd64.zip
sblite-v0.2.7-linux-arm64.zip
sblite-v0.2.7-linux-amd64.zip
sblite-v0.2.7-windows-arm64.zip
sblite-v0.2.7-windows-amd64.zip
checksums.txt
```

### Edge Runtime Release (e.g., edge-runtime-v1.67.4)

```
edge-runtime-v1.67.4-darwin-arm64
edge-runtime-v1.67.4-darwin-amd64
edge-runtime-v1.67.4-linux-arm64
edge-runtime-v1.67.4-linux-amd64
checksums.txt
```

## Troubleshooting

### Build Failures

**sblite builds fail:**
- Check Go version compatibility (requires Go 1.25+)
- Verify no CGO dependencies were introduced

**Edge runtime builds fail:**
- Check Rust toolchain installation on runners
- Verify supabase/edge-runtime tag exists
- Review build logs for missing dependencies

### Checksum Mismatches

If users report checksum errors:
1. Download the binary manually
2. Run `sha256sum <binary>`
3. Compare with `checksums.txt` in the release
4. If different, the release may be corrupted - rebuild

### Platform Not Supported

If a user's platform isn't supported:
- Windows: Direct them to Docker instructions
- Other platforms: They can build from source

## Manual Builds

For local testing or unsupported platforms:

### sblite

```bash
# Build for current platform
go build -o sblite .

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o sblite-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o sblite-darwin-arm64 .
```

### Edge Runtime

```bash
# Clone and build
git clone https://github.com/supabase/edge-runtime
cd edge-runtime
git checkout v1.67.4
cargo build --release

# Binary at target/release/edge-runtime
```
