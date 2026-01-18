package dashboard

import (
	"errors"
	"path/filepath"
	"strings"
)

// MaxFileSize is the maximum allowed file size for function files (1MB)
const MaxFileSize = 1 * 1024 * 1024

// allowedExtensions contains the set of permitted file extensions for function files
var allowedExtensions = map[string]bool{
	".ts":   true,
	".js":   true,
	".json": true,
	".mjs":  true,
	".tsx":  true,
	".jsx":  true,
	".html": true,
	".css":  true,
	".md":   true,
	".txt":  true,
}

// Errors returned by path validation functions
var (
	ErrEmptyPath          = errors.New("path cannot be empty")
	ErrAbsolutePath       = errors.New("absolute paths are not allowed")
	ErrPathTraversal      = errors.New("path traversal is not allowed")
	ErrHiddenFile         = errors.New("hidden files are not allowed")
	ErrDisallowedExtension = errors.New("file extension is not allowed")
	ErrPathEscapesBase    = errors.New("path escapes base directory")
)

// ValidateFunctionFilePath validates a relative file path for security.
// It rejects empty paths, absolute paths, path traversal attempts,
// hidden files, and files with disallowed extensions.
func ValidateFunctionFilePath(path string) error {
	// Check for empty path
	if path == "" {
		return ErrEmptyPath
	}

	// Check for absolute paths (Unix and Windows)
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") || (len(path) >= 2 && path[1] == ':') {
		return ErrAbsolutePath
	}

	// Check for path traversal attempts
	// This catches ../ and ..\ patterns as well as URL-encoded versions
	if containsPathTraversal(path) {
		return ErrPathTraversal
	}

	// Check for hidden files (starting with .)
	if containsHiddenFile(path) {
		return ErrHiddenFile
	}

	// Check file extension
	ext := filepath.Ext(path)
	if !IsAllowedExtension(ext) {
		return ErrDisallowedExtension
	}

	return nil
}

// containsPathTraversal checks if the path contains any path traversal patterns
func containsPathTraversal(path string) bool {
	// Normalize path separators for checking
	normalized := strings.ReplaceAll(path, "\\", "/")

	// Check for URL-encoded traversal attempts
	if strings.Contains(path, "%2F") || strings.Contains(path, "%2f") ||
		strings.Contains(path, "%5C") || strings.Contains(path, "%5c") {
		return true
	}

	// Split and check each component
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
		// Also check for backslash variants within parts
		if strings.Contains(part, "..") {
			return true
		}
	}

	return false
}

// containsHiddenFile checks if any component of the path is a hidden file/directory
func containsHiddenFile(path string) bool {
	// Normalize path separators
	normalized := strings.ReplaceAll(path, "\\", "/")

	// Split and check each component
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}

	return false
}

// IsAllowedExtension checks if the given file extension is in the allowed list.
// The extension should include the leading dot (e.g., ".ts", ".js").
func IsAllowedExtension(ext string) bool {
	return allowedExtensions[strings.ToLower(ext)]
}

// SanitizePath validates a relative path and joins it with a base path,
// ensuring the result stays within the base directory.
// Returns the sanitized absolute path or an error if validation fails.
func SanitizePath(basePath, relativePath string) (string, error) {
	// First validate the relative path
	if err := ValidateFunctionFilePath(relativePath); err != nil {
		return "", err
	}

	// Clean both paths
	cleanBase := filepath.Clean(basePath)
	cleanRelative := filepath.Clean(relativePath)

	// Join the paths
	fullPath := filepath.Join(cleanBase, cleanRelative)

	// Ensure the result is still within the base directory
	// by checking that the full path starts with the base path
	if !strings.HasPrefix(fullPath, cleanBase+string(filepath.Separator)) &&
		fullPath != cleanBase {
		return "", ErrPathEscapesBase
	}

	return fullPath, nil
}
