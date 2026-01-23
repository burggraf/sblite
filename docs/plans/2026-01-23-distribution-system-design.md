# Distribution System Design

## Overview

This document describes the build and distribution system for sblite and the edge runtime across multiple platforms and architectures.

## Target Platforms

### sblite Binary (6 platforms)
- darwin-arm64 (macOS Apple Silicon)
- darwin-amd64 (macOS Intel)
- linux-arm64
- linux-amd64
- windows-arm64
- windows-amd64

### Edge Runtime (4 platforms, no Windows native)
- darwin-arm64
- darwin-amd64
- linux-arm64
- linux-amd64

Windows users receive Docker instructions as a fallback.

## Architecture

### 1. sblite Binary Distribution

**GitHub Actions workflow** triggered on version tags (e.g., `v0.2.7`).

Build matrix:

| Runner          | GOOS    | GOARCH | Output                          |
|-----------------|---------|--------|----------------------------------|
| ubuntu-latest   | linux   | amd64  | sblite-v0.2.7-linux-amd64.zip   |
| ubuntu-latest   | linux   | arm64  | sblite-v0.2.7-linux-arm64.zip   |
| ubuntu-latest   | windows | amd64  | sblite-v0.2.7-windows-amd64.zip |
| ubuntu-latest   | windows | arm64  | sblite-v0.2.7-windows-arm64.zip |
| macos-latest    | darwin  | arm64  | sblite-v0.2.7-darwin-arm64.zip  |
| macos-latest    | darwin  | amd64  | sblite-v0.2.7-darwin-amd64.zip  |

Go cross-compiles easily since sblite uses `modernc.org/sqlite` (pure Go, no CGO).

### 2. Edge Runtime Distribution

**Built from source**: Compiled from `supabase/edge-runtime` repository.

**Hosted on sblite GitHub releases** with tag namespace `edge-runtime-v{version}`.

**Manual trigger workflow** since edge runtime version changes rarely.

Build matrix (native builds to avoid V8/Deno cross-compilation issues):

| Runner              | Target            | Output                              |
|---------------------|-------------------|-------------------------------------|
| ubuntu-latest       | x86_64-linux-gnu  | edge-runtime-v1.67.4-linux-amd64    |
| ubuntu-24.04-arm64  | aarch64-linux-gnu | edge-runtime-v1.67.4-linux-arm64    |
| macos-latest        | aarch64-apple     | edge-runtime-v1.67.4-darwin-arm64   |
| macos-13            | x86_64-apple      | edge-runtime-v1.67.4-darwin-amd64   |

### 3. Dashboard Runtime Installer

Auto-detects platform/architecture and downloads from GitHub releases with progress indicator.

## API Design

### GET /_/api/functions/runtime-info

Returns runtime availability and download information.

**Supported platform response:**
```json
{
  "installed": false,
  "available": true,
  "platform": "darwin-arm64",
  "version": "v1.67.4",
  "download_url": "https://github.com/burggraf/sblite/releases/download/edge-runtime-v1.67.4/edge-runtime-v1.67.4-darwin-arm64",
  "checksum": "abc123...",
  "install_path": "/path/to/edge-runtime/",
  "size_bytes": 45000000
}
```

**Windows (unsupported) response:**
```json
{
  "installed": false,
  "available": false,
  "platform": "windows-amd64",
  "fallback": "docker",
  "docker_command": "docker run -d -p 9000:9000 -v ./functions:/functions ghcr.io/supabase/edge-runtime:v1.67.4 start --main-service /functions"
}
```

### POST /_/api/functions/runtime-install

Triggers download with progress via Server-Sent Events (SSE):

```
data: {"status": "downloading", "progress": 45, "bytes_downloaded": 20000000}
data: {"status": "verifying", "progress": 95}
data: {"status": "complete", "path": "/path/to/edge-runtime-v1.67.4"}
```

## Dashboard UI States

### State 1: Functions disabled
Server started without `--functions` flag. Shows existing message to restart with flag.

### State 2: Runtime not installed, platform supported
Shows download button with platform info, version, and size. Progress bar appears during download.

### State 3: Platform not supported (Windows)
Shows Docker instructions with copyable command.

### State 4: Runtime installed
Current behavior - shows function list and runtime status indicator.

## Checksum Management

SHA256 checksums embedded in `internal/functions/download.go`:

```go
var edgeRuntimeChecksums = map[string]map[string]string{
    "v1.67.4": {
        "darwin-arm64": "a1b2c3d4e5f6...",
        "darwin-amd64": "b2c3d4e5f6a1...",
        "linux-arm64":  "c3d4e5f6a1b2...",
        "linux-amd64":  "d4e5f6a1b2c3...",
    },
}
```

When upgrading edge runtime version:
1. Run edge-runtime build workflow for new version
2. Copy checksums from release's `checksums.txt`
3. Update `EdgeRuntimeVersion` constant and checksums map
4. Release new sblite version

## GitHub Release Structure

### sblite releases (tag: v0.2.7)
```
sblite-v0.2.7-darwin-arm64.zip
sblite-v0.2.7-darwin-amd64.zip
sblite-v0.2.7-linux-arm64.zip
sblite-v0.2.7-linux-amd64.zip
sblite-v0.2.7-windows-arm64.zip
sblite-v0.2.7-windows-amd64.zip
checksums.txt
```

### edge-runtime releases (tag: edge-runtime-v1.67.4)
```
edge-runtime-v1.67.4-darwin-arm64
edge-runtime-v1.67.4-darwin-amd64
edge-runtime-v1.67.4-linux-arm64
edge-runtime-v1.67.4-linux-amd64
checksums.txt
```

## Implementation Plan

### Files to Create

| File | Purpose |
|------|---------|
| `.github/workflows/release.yml` | sblite cross-platform build workflow |
| `.github/workflows/edge-runtime.yml` | Edge runtime build workflow (manual trigger) |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/functions/download.go` | Replace GHCR extraction with GitHub release downloads, add checksums map, add progress reporting |
| `internal/dashboard/handler.go` | Add `GET /runtime-info` and `POST /runtime-install` endpoints |
| `internal/dashboard/static/app.js` | Add runtime installer UI with progress bar, Docker fallback for Windows |
| `internal/dashboard/static/style.css` | Styles for installer UI, progress bar |

### Implementation Order

1. Create sblite release workflow (immediate value, automates current manual process)
2. Create edge-runtime build workflow (one-time setup)
3. Run edge-runtime workflow (produces binaries + checksums)
4. Update download.go (new download source, checksums)
5. Add dashboard API endpoints (runtime-info, runtime-install with SSE)
6. Update dashboard UI (installer states, progress bar)

### Dependencies

- Edge runtime workflow must complete before updating `download.go` (need actual checksums)
- sblite workflow can be created and used immediately
