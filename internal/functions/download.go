package functions

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// GHCR registry URL
	ghcrRegistry = "ghcr.io"
	// Edge runtime image name
	edgeRuntimeImage = "supabase/edge-runtime"
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

// Download downloads and extracts the edge-runtime binary from GHCR.
func (d *Downloader) Download() error {
	// Ensure download directory exists
	if err := os.MkdirAll(d.downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	log.Info("downloading edge runtime from GHCR",
		"version", d.version,
		"platform", platform,
	)

	// Step 1: Get auth token from GHCR
	token, err := d.getGHCRToken()
	if err != nil {
		return fmt.Errorf("failed to get GHCR token: %w", err)
	}

	// Step 2: Get image manifest
	manifest, err := d.getImageManifest(token)
	if err != nil {
		return fmt.Errorf("failed to get image manifest: %w", err)
	}

	// Step 3: Download and extract layers to find the binary
	binaryPath := d.BinaryPath()
	if err := d.extractBinaryFromLayers(token, manifest, binaryPath); err != nil {
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

// getGHCRToken gets an anonymous auth token from GHCR for pulling public images.
func (d *Downloader) getGHCRToken() (string, error) {
	url := fmt.Sprintf("https://%s/token?scope=repository:%s:pull", ghcrRegistry, edgeRuntimeImage)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResp.Token, nil
}

// manifestList represents a Docker manifest list (fat manifest).
type manifestList struct {
	SchemaVersion int `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Manifests     []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
		Platform  struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

// imageManifest represents a Docker image manifest.
type imageManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	} `json:"layers"`
}

// getImageManifest gets the image manifest for the current platform.
func (d *Downloader) getImageManifest(token string) (*imageManifest, error) {
	// First, try to get the manifest list (multi-arch image)
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ghcrRegistry, edgeRuntimeImage, d.version)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// Accept both manifest list and single manifest
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("manifest request failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Check if it's a manifest list
	if strings.Contains(contentType, "manifest.list") || strings.Contains(contentType, "image.index") {
		var list manifestList
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, fmt.Errorf("failed to parse manifest list: %w", err)
		}

		// Find the manifest for our platform
		targetArch := runtime.GOARCH
		targetOS := runtime.GOOS

		var digest string
		for _, m := range list.Manifests {
			if m.Platform.OS == targetOS && m.Platform.Architecture == targetArch {
				digest = m.Digest
				break
			}
		}

		if digest == "" {
			return nil, fmt.Errorf("no manifest found for platform %s/%s", targetOS, targetArch)
		}

		// Fetch the platform-specific manifest
		return d.fetchManifestByDigest(token, digest)
	}

	// It's a single manifest
	var manifest imageManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// fetchManifestByDigest fetches a manifest by its digest.
func (d *Downloader) fetchManifestByDigest(token, digest string) (*imageManifest, error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ghcrRegistry, edgeRuntimeImage, digest)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("manifest request failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var manifest imageManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// extractBinaryFromLayers downloads layers and extracts the edge-runtime binary.
func (d *Downloader) extractBinaryFromLayers(token string, manifest *imageManifest, destPath string) error {
	// Process layers in reverse order (top layer first, more likely to have the binary)
	for i := len(manifest.Layers) - 1; i >= 0; i-- {
		layer := manifest.Layers[i]
		log.Debug("checking layer for edge-runtime binary", "layer", i, "digest", layer.Digest[:20]+"...")

		found, err := d.searchLayerForBinary(token, layer.Digest, destPath)
		if err != nil {
			log.Debug("error searching layer", "error", err)
			continue
		}
		if found {
			return nil
		}
	}

	return fmt.Errorf("edge-runtime binary not found in any layer")
}

// searchLayerForBinary downloads a layer and searches for the edge-runtime binary.
func (d *Downloader) searchLayerForBinary(token, digest, destPath string) (bool, error) {
	url := fmt.Sprintf("https://%s/v2/%s/blobs/%s", ghcrRegistry, edgeRuntimeImage, digest)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("blob request failed: HTTP %d", resp.StatusCode)
	}

	// Layers are gzipped tarballs
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Look for the edge-runtime binary
		// Common paths: /usr/local/bin/edge-runtime, /edge-runtime, etc.
		name := header.Name
		if strings.HasSuffix(name, "/edge-runtime") || name == "edge-runtime" ||
			strings.HasSuffix(name, "/edge-runtime-server") || name == "edge-runtime-server" ||
			name == "usr/local/bin/edge-runtime" {

			log.Info("found edge-runtime binary in layer", "path", name)

			// Extract to destination
			out, err := os.Create(destPath)
			if err != nil {
				return false, fmt.Errorf("failed to create output file: %w", err)
			}

			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				os.Remove(destPath)
				return false, fmt.Errorf("failed to extract binary: %w", err)
			}

			return true, nil
		}
	}

	return false, nil
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
