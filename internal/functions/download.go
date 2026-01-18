package functions

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/markb/sblite/internal/log"
)

const (
	// EdgeRuntimeVersion is the pinned version of edge-runtime to use.
	EdgeRuntimeVersion = "v1.67.4"
	// EdgeRuntimeBaseURL is the GitHub releases URL for edge-runtime.
	EdgeRuntimeBaseURL = "https://github.com/supabase/edge-runtime/releases/download"
)

// SHA256 checksums for each platform binary.
// These should be updated when EdgeRuntimeVersion changes.
// To get checksums, download each binary and run: shasum -a 256 <file>
var checksums = map[string]string{
	// Note: These are placeholder checksums - they need to be verified
	// against actual releases. When a checksum is empty, verification is skipped.
	"darwin-amd64": "",
	"darwin-arm64": "",
	"linux-amd64":  "",
	"linux-arm64":  "",
}

// Downloader handles downloading and verifying the edge-runtime binary.
type Downloader struct {
	downloadDir string
	version     string
}

// NewDownloader creates a new downloader.
func NewDownloader(downloadDir string) *Downloader {
	return &Downloader{
		downloadDir: downloadDir,
		version:     EdgeRuntimeVersion,
	}
}

// DefaultDownloadDir returns the default directory for downloaded binaries.
func DefaultDownloadDir() string {
	// Use XDG_DATA_HOME if available, otherwise ~/.local/share
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

// EnsureBinary ensures the edge-runtime binary is downloaded and verified.
func (d *Downloader) EnsureBinary() (string, error) {
	binaryPath := d.BinaryPath()

	// Check if binary already exists and is valid
	if _, err := os.Stat(binaryPath); err == nil {
		// Binary exists, verify checksum if we have one
		key := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
		if expected := checksums[key]; expected != "" {
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

// Download downloads and extracts the edge-runtime binary.
func (d *Downloader) Download() error {
	// Ensure download directory exists
	if err := os.MkdirAll(d.downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	url := d.getBinaryURL()
	log.Info("downloading edge runtime",
		"version", d.version,
		"url", url,
		"platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	)

	// Download the tarball
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download edge runtime: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download edge runtime: HTTP %d", resp.StatusCode)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp(d.downloadDir, "edge-runtime-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy download to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to save download: %w", err)
	}

	// Extract the binary
	binaryPath := d.BinaryPath()
	if err := extractBinary(tmpPath, binaryPath); err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Verify checksum if available
	key := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	if expected := checksums[key]; expected != "" {
		if err := verifyChecksum(binaryPath, expected); err != nil {
			os.Remove(binaryPath)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	log.Info("edge runtime downloaded successfully", "path", binaryPath)
	return nil
}

// getBinaryURL returns the download URL for the current platform.
func (d *Downloader) getBinaryURL() string {
	// Map Go arch to edge-runtime naming
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	// Map Go OS to edge-runtime naming
	osName := runtime.GOOS
	switch osName {
	case "darwin":
		osName = "apple-darwin"
	case "linux":
		osName = "unknown-linux-gnu"
	}

	filename := fmt.Sprintf("edge-runtime_%s_%s-%s.tar.gz", d.version, arch, osName)
	return fmt.Sprintf("%s/%s/%s", EdgeRuntimeBaseURL, d.version, filename)
}

// extractBinary extracts the edge-runtime binary from a tarball.
func extractBinary(tarPath, destPath string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Look for the edge-runtime binary
		name := header.Name
		if strings.HasSuffix(name, "/edge-runtime") || name == "edge-runtime" {
			// Extract to destination
			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("failed to extract binary: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("edge-runtime binary not found in archive")
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

// IsSupported returns true if the current platform is supported.
func IsSupported() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		switch runtime.GOARCH {
		case "amd64", "arm64":
			return true
		}
	}
	return false
}

// PlatformString returns a human-readable platform string.
func PlatformString() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
