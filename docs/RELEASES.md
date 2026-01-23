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

## macOS Code Signing Setup

macOS binaries are signed and notarized to avoid Gatekeeper warnings. This requires an Apple Developer account and GitHub repository secrets.

### Prerequisites

1. **Apple Developer Program membership** ($99/year)
2. **Developer ID Application certificate** from Apple Developer portal

### Creating the Certificate

1. Open **Keychain Access** on your Mac
2. Go to **Keychain Access → Certificate Assistant → Request a Certificate from a Certificate Authority**
3. Enter your email and name, select "Saved to disk"
4. Log in to [Apple Developer Portal](https://developer.apple.com/account)
5. Go to **Certificates, Identifiers & Profiles → Certificates**
6. Click **+** to create a new certificate
7. Select **Developer ID Application** (for distributing outside App Store)
8. Upload your certificate signing request
9. Download the certificate and double-click to install in Keychain

### Exporting as P12

1. Open **Keychain Access**
2. Find your "Developer ID Application" certificate
3. Right-click → **Export**
4. Choose **Personal Information Exchange (.p12)** format
5. Set a strong password (you'll need this for GitHub secrets)
6. Save the file

### Creating App-Specific Password

1. Go to [appleid.apple.com](https://appleid.apple.com)
2. Sign in and go to **Sign-In and Security → App-Specific Passwords**
3. Click **Generate an app-specific password**
4. Name it "sblite-notarization"
5. Copy the generated password

### Finding Your Team ID

1. Go to [Apple Developer Portal](https://developer.apple.com/account)
2. Click **Membership Details** in the sidebar
3. Your Team ID is listed there (10-character alphanumeric)

### Configuring GitHub Secrets

Go to your repository **Settings → Secrets and variables → Actions** and add:

| Secret | Value |
|--------|-------|
| `APPLE_CERTIFICATE_P12` | Base64-encoded .p12 file: `base64 -i certificate.p12 \| pbcopy` |
| `APPLE_CERTIFICATE_PASSWORD` | Password you set when exporting the .p12 |
| `APPLE_TEAM_ID` | Your 10-character Team ID |
| `APPLE_ID` | Your Apple ID email address |
| `APPLE_APP_PASSWORD` | App-specific password from appleid.apple.com |

### Testing Locally

You can test signing locally before pushing:

```bash
# Build the binary
go build -o sblite .

# Sign with your Developer ID
codesign --force --options runtime \
  --sign "Developer ID Application: Your Name (TEAM_ID)" \
  --timestamp \
  sblite

# Verify signature
codesign --verify --verbose sblite

# Create zip for notarization
zip sblite.zip sblite

# Submit for notarization
xcrun notarytool submit sblite.zip \
  --apple-id "your@email.com" \
  --password "app-specific-password" \
  --team-id "TEAM_ID" \
  --wait
```

### Troubleshooting

**"No identity found"**
- Ensure the certificate is installed in your login keychain
- Run `security find-identity -v -p codesigning` to list available identities

**Notarization rejected**
- Check the notarization log: `xcrun notarytool log <submission-id> --apple-id ... --password ... --team-id ...`
- Common issues: hardened runtime not enabled, unsigned embedded binaries

**Certificate expired**
- Certificates are valid for 5 years
- Create a new certificate and update the GitHub secret

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
