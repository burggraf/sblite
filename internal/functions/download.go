package functions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/markb/sblite/internal/log"
)

const (
	// EdgeRuntimeVersion is the pinned version of edge-runtime to use.
	EdgeRuntimeVersion = "v1.67.4"

	// GitHubReleaseBaseURL is the base URL for edge-runtime releases on GitHub.
	GitHubReleaseBaseURL = "https://github.com/burggraf/sblite/releases/download"
)

// SHA256 checksums for each platform binary, keyed by version then platform.
// These are updated when EdgeRuntimeVersion changes by running the edge-runtime workflow.
// To get checksums: download binaries and run `sha256sum <file>`
var edgeRuntimeChecksums = map[string]map[string]string{
	"v1.67.4": {
		"darwin-amd64": "c31ad1bb0081c7368de0864a9673eac46d20f6bf8888450f177eeb12a4d7db70",
		"darwin-arm64": "f7614de93d4e0d0175a899dc0f1f877335364e92df5f637941a22743c020b815",
		"linux-amd64":  "e9a6fffdffd655b694d7897cd3c462d3c8e1afce67e1b8a3892b5beb99d988f7",
		"linux-arm64":  "", // Coming soon - ARM runner was unavailable
	},
}

// Approximate binary sizes in bytes for progress estimation (updated after builds).
var edgeRuntimeSizes = map[string]int64{
	"darwin-amd64": 148 * 1024 * 1024,  // ~148MB
	"darwin-arm64": 144 * 1024 * 1024,  // ~144MB
	"linux-amd64":  1020 * 1024 * 1024, // ~1GB (includes debug symbols)
	"linux-arm64":  1020 * 1024 * 1024, // ~1GB estimated
}

// ProgressCallback is called during download with progress updates.
type ProgressCallback func(bytesDownloaded, totalBytes int64)

// RuntimeInfo contains information about the edge runtime for a platform.
type RuntimeInfo struct {
	Installed     bool   `json:"installed"`
	Available     bool   `json:"available"`
	Platform      string `json:"platform"`
	Version       string `json:"version"`
	DownloadURL   string `json:"download_url,omitempty"`
	Checksum      string `json:"checksum,omitempty"`
	InstallPath   string `json:"install_path,omitempty"`
	SizeBytes     int64  `json:"size_bytes,omitempty"`
	Fallback      string `json:"fallback,omitempty"`
	DockerCommand string `json:"docker_command,omitempty"`
}

// Downloader handles downloading and verifying the edge-runtime binary.
type Downloader struct {
	downloadDir      string
	version          string
	progressCallback ProgressCallback
}

// NewDownloader creates a new downloader.
func NewDownloader(downloadDir string) *Downloader {
	return &Downloader{
		downloadDir: downloadDir,
		version:     EdgeRuntimeVersion,
	}
}

// SetProgressCallback sets a callback function for download progress updates.
func (d *Downloader) SetProgressCallback(cb ProgressCallback) {
	d.progressCallback = cb
}

// DefaultDownloadDir returns the default directory for downloaded binaries.
// If dbPath is provided, returns <db-dir>/edge-runtime/
// Otherwise falls back to XDG_DATA_HOME/sblite/bin/ or ~/.local/share/sblite/bin/
func DefaultDownloadDir(dbPath string) string {
	// If database path provided, use sibling edge-runtime directory
	if dbPath != "" {
		dbDir := filepath.Dir(dbPath)
		// Handle relative paths
		if absDir, err := filepath.Abs(dbDir); err == nil {
			dbDir = absDir
		}
		return filepath.Join(dbDir, "edge-runtime")
	}

	// Fallback: Use XDG_DATA_HOME if available, otherwise ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "sblite", "bin")
}

// BinaryPath returns the path where the binary will be/is stored.
func (d *Downloader) BinaryPath() string {
	return filepath.Join(d.downloadDir, fmt.Sprintf("edge-runtime-%s", d.version))
}

// DownloadURL returns the GitHub release download URL for the current platform.
func (d *Downloader) DownloadURL() string {
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	return fmt.Sprintf("%s/edge-runtime-%s/edge-runtime-%s-%s",
		GitHubReleaseBaseURL, d.version, d.version, platform)
}

// GetChecksum returns the expected checksum for the current platform and version.
func (d *Downloader) GetChecksum() string {
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	if versionChecksums, ok := edgeRuntimeChecksums[d.version]; ok {
		return versionChecksums[platform]
	}
	return ""
}

// GetEstimatedSize returns the estimated binary size for the current platform.
func (d *Downloader) GetEstimatedSize() int64 {
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	if size, ok := edgeRuntimeSizes[platform]; ok {
		return size
	}
	return 50 * 1024 * 1024 // Default 50MB
}

// IsInstalled checks if the edge runtime binary is already installed.
func (d *Downloader) IsInstalled() bool {
	binaryPath := d.BinaryPath()
	if _, err := os.Stat(binaryPath); err != nil {
		return false
	}

	// Verify checksum if available
	if expected := d.GetChecksum(); expected != "" {
		if err := verifyChecksum(binaryPath, expected); err != nil {
			return false
		}
	}
	return true
}

// GetRuntimeInfo returns information about the edge runtime for the current platform.
func GetRuntimeInfo(downloadDir string) *RuntimeInfo {
	d := NewDownloader(downloadDir)
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	info := &RuntimeInfo{
		Platform:    platform,
		Version:     EdgeRuntimeVersion,
		InstallPath: d.downloadDir,
	}

	// Check if installed
	info.Installed = d.IsInstalled()

	// Check if platform is supported
	if IsSupported() {
		info.Available = true
		info.DownloadURL = d.DownloadURL()
		info.Checksum = d.GetChecksum()
		info.SizeBytes = d.GetEstimatedSize()
	} else if runtime.GOOS == "windows" {
		// Windows gets Docker fallback
		info.Available = false
		info.Fallback = "docker"
		info.DockerCommand = fmt.Sprintf(
			"docker run -d -p 9000:9000 -v ./functions:/functions ghcr.io/supabase/edge-runtime:%s start --main-service /functions",
			EdgeRuntimeVersion,
		)
	} else {
		info.Available = false
	}

	return info
}

// EnsureBinary ensures the edge-runtime binary is downloaded and verified.
func (d *Downloader) EnsureBinary() (string, error) {
	binaryPath := d.BinaryPath()

	// Check if binary already exists and is valid
	if _, err := os.Stat(binaryPath); err == nil {
		// Binary exists, verify checksum if we have one
		if expected := d.GetChecksum(); expected != "" {
			if err := verifyChecksum(binaryPath, expected); err != nil {
				log.Warn("existing binary checksum mismatch, re-downloading", "error", err)
				os.Remove(binaryPath)
			} else {
				return binaryPath, nil
			}
		} else {
			// No checksum available, assume valid
			return binaryPath, nil
		}
	}

	// Download the binary
	if err := d.Download(); err != nil {
		return "", err
	}

	return binaryPath, nil
}

// Download downloads the edge-runtime binary from GitHub releases.
func (d *Downloader) Download() error {
	// Ensure download directory exists
	if err := os.MkdirAll(d.downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	downloadURL := d.DownloadURL()

	log.Info("downloading edge runtime from GitHub",
		"version", d.version,
		"platform", platform,
		"url", downloadURL,
	)

	// Create HTTP request
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Get content length for progress
	totalBytes := resp.ContentLength
	if totalBytes <= 0 {
		totalBytes = d.GetEstimatedSize()
	}

	// Create temporary file
	binaryPath := d.BinaryPath()
	tmpPath := binaryPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		out.Close()
		os.Remove(tmpPath) // Clean up on error
	}()

	// Download with progress reporting
	var bytesDownloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write: %w", writeErr)
			}
			bytesDownloaded += int64(n)

			if d.progressCallback != nil {
				d.progressCallback(bytesDownloaded, totalBytes)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("failed to read: %w", readErr)
		}
	}

	out.Close()

	// Verify checksum if available
	if expected := d.GetChecksum(); expected != "" {
		if err := verifyChecksum(tmpPath, expected); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Move to final location
	if err := os.Rename(tmpPath, binaryPath); err != nil {
		return fmt.Errorf("failed to move binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	log.Info("edge runtime downloaded successfully", "path", binaryPath)
	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file.
func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// IsSupported returns true if the current platform is supported for edge runtime.
// Supported: Linux (amd64, arm64) and macOS (amd64, arm64).
// Windows is not supported (users should use Docker).
func IsSupported() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		switch runtime.GOARCH {
		case "amd64", "arm64":
			return true
		}
	}
	return false
}

// UnsupportedPlatformError returns a helpful error message for unsupported platforms.
func UnsupportedPlatformError() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("edge functions on Windows require Docker.\n\nRun this command to start the edge runtime:\n  docker run -d -p 9000:9000 -v ./functions:/functions ghcr.io/supabase/edge-runtime:%s start --main-service /functions", EdgeRuntimeVersion)
	}
	return fmt.Errorf("edge functions not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

// PlatformString returns a human-readable platform string.
func PlatformString() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}
