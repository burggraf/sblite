package backend

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalBackend implements Backend using the local filesystem.
type LocalBackend struct {
	basePath string
}

// NewLocal creates a new local filesystem backend.
// basePath is the root directory for storing files.
// The directory will be created if it doesn't exist.
func NewLocal(basePath string) (*LocalBackend, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, &Error{Op: "NewLocal", Err: fmt.Errorf("invalid path: %w", err)}
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, &Error{Op: "NewLocal", Err: fmt.Errorf("create directory: %w", err)}
	}

	return &LocalBackend{basePath: absPath}, nil
}

// validateKey checks if a key is safe to use.
// Prevents path traversal attacks and invalid paths.
func (b *LocalBackend) validateKey(key string) error {
	if key == "" {
		return &Error{Op: "validateKey", Key: key, Err: errInvalidKey{}}
	}

	// Check for null bytes
	if strings.ContainsRune(key, 0) {
		return &Error{Op: "validateKey", Key: key, Err: errInvalidKey{}}
	}

	// Check for path traversal
	if strings.Contains(key, "..") {
		return &Error{Op: "validateKey", Key: key, Err: errInvalidKey{}}
	}

	// Check for absolute paths
	if filepath.IsAbs(key) {
		return &Error{Op: "validateKey", Key: key, Err: errInvalidKey{}}
	}

	// Clean and verify the path stays within base
	cleaned := filepath.Clean(key)
	if strings.HasPrefix(cleaned, "..") {
		return &Error{Op: "validateKey", Key: key, Err: errInvalidKey{}}
	}

	return nil
}

// fullPath returns the full filesystem path for a key.
func (b *LocalBackend) fullPath(key string) string {
	return filepath.Join(b.basePath, filepath.FromSlash(key))
}

// Exists checks if a file exists at the given key.
func (b *LocalBackend) Exists(ctx context.Context, key string) (bool, error) {
	if err := b.validateKey(key); err != nil {
		return false, err
	}

	_, err := os.Stat(b.fullPath(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, &Error{Op: "Exists", Key: key, Err: err}
}

// Attributes returns metadata for a file.
func (b *LocalBackend) Attributes(ctx context.Context, key string) (*FileInfo, error) {
	if err := b.validateKey(key); err != nil {
		return nil, err
	}

	path := b.fullPath(key)
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &Error{Op: "Attributes", Key: key, Err: errNotFound{}}
		}
		return nil, &Error{Op: "Attributes", Key: key, Err: err}
	}

	if stat.IsDir() {
		return nil, &Error{Op: "Attributes", Key: key, Err: errNotFound{}}
	}

	// Calculate ETag from file content
	etag, err := b.calculateETag(path)
	if err != nil {
		return nil, &Error{Op: "Attributes", Key: key, Err: fmt.Errorf("calculate etag: %w", err)}
	}

	return &FileInfo{
		Key:         key,
		Size:        stat.Size(),
		ContentType: "", // Content type not stored on filesystem; caller should track separately
		ETag:        etag,
		ModTime:     stat.ModTime(),
	}, nil
}

// calculateETag computes MD5 hash of file for ETag.
func (b *LocalBackend) calculateETag(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Reader returns a reader for the file content.
func (b *LocalBackend) Reader(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error) {
	if err := b.validateKey(key); err != nil {
		return nil, nil, err
	}

	path := b.fullPath(key)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, &Error{Op: "Reader", Key: key, Err: errNotFound{}}
		}
		return nil, nil, &Error{Op: "Reader", Key: key, Err: err}
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, &Error{Op: "Reader", Key: key, Err: err}
	}

	if stat.IsDir() {
		f.Close()
		return nil, nil, &Error{Op: "Reader", Key: key, Err: errNotFound{}}
	}

	info := &FileInfo{
		Key:     key,
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}

	return f, info, nil
}

// Write stores content at the given key.
func (b *LocalBackend) Write(ctx context.Context, key string, content io.Reader, size int64, contentType string) (*FileInfo, error) {
	if err := b.validateKey(key); err != nil {
		return nil, err
	}

	path := b.fullPath(key)

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, &Error{Op: "Write", Key: key, Err: fmt.Errorf("create directory: %w", err)}
	}

	// Write to temp file first, then rename for atomicity
	tmpFile, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return nil, &Error{Op: "Write", Key: key, Err: fmt.Errorf("create temp file: %w", err)}
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temp file on error
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Write content and calculate ETag simultaneously
	h := md5.New()
	writer := io.MultiWriter(tmpFile, h)

	var written int64
	if size >= 0 {
		written, err = io.CopyN(writer, content, size)
	} else {
		written, err = io.Copy(writer, content)
	}
	if err != nil {
		return nil, &Error{Op: "Write", Key: key, Err: fmt.Errorf("write content: %w", err)}
	}

	// Close temp file before rename
	if err := tmpFile.Close(); err != nil {
		return nil, &Error{Op: "Write", Key: key, Err: fmt.Errorf("close temp file: %w", err)}
	}
	tmpFile = nil // Prevent cleanup in defer

	// Rename to final location
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return nil, &Error{Op: "Write", Key: key, Err: fmt.Errorf("rename to final: %w", err)}
	}

	etag := hex.EncodeToString(h.Sum(nil))

	return &FileInfo{
		Key:         key,
		Size:        written,
		ContentType: contentType,
		ETag:        etag,
		ModTime:     time.Now(),
	}, nil
}

// Delete removes a file at the given key.
func (b *LocalBackend) Delete(ctx context.Context, key string) error {
	if err := b.validateKey(key); err != nil {
		return err
	}

	path := b.fullPath(key)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return &Error{Op: "Delete", Key: key, Err: err}
	}

	// Try to remove empty parent directories
	b.cleanEmptyDirs(filepath.Dir(path))

	return nil
}

// cleanEmptyDirs removes empty directories up to basePath.
func (b *LocalBackend) cleanEmptyDirs(dir string) {
	for dir != b.basePath && strings.HasPrefix(dir, b.basePath) {
		err := os.Remove(dir)
		if err != nil {
			break // Directory not empty or other error
		}
		dir = filepath.Dir(dir)
	}
}

// DeletePrefix removes all files with the given prefix.
func (b *LocalBackend) DeletePrefix(ctx context.Context, prefix string) []error {
	if prefix != "" {
		if err := b.validateKey(prefix); err != nil {
			return []error{err}
		}
	}

	var errors []error
	basePath := b.basePath
	if prefix != "" {
		basePath = b.fullPath(prefix)
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			errors = append(errors, &Error{Op: "DeletePrefix", Key: prefix, Err: err})
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			relPath, _ := filepath.Rel(b.basePath, path)
			errors = append(errors, &Error{Op: "DeletePrefix", Key: relPath, Err: err})
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		errors = append(errors, &Error{Op: "DeletePrefix", Key: prefix, Err: err})
	}

	// Clean up empty directories
	if prefix != "" {
		b.cleanEmptyDirs(basePath)
	}

	return errors
}

// List returns files with the given prefix.
func (b *LocalBackend) List(ctx context.Context, prefix string, limit int, cursor string) ([]FileInfo, string, error) {
	if prefix != "" {
		if err := b.validateKey(prefix); err != nil {
			return nil, "", err
		}
	}

	if limit <= 0 {
		limit = 1000 // Default limit
	}

	var files []FileInfo
	basePath := b.basePath

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path as key
		relPath, err := filepath.Rel(b.basePath, path)
		if err != nil {
			return nil
		}

		// Convert to forward slashes for consistency
		key := filepath.ToSlash(relPath)

		// Check prefix filter
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		// Check cursor (skip keys <= cursor)
		if cursor != "" && key <= cursor {
			return nil
		}

		files = append(files, FileInfo{
			Key:     key,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, "", &Error{Op: "List", Key: prefix, Err: err}
	}

	// Sort by key for consistent ordering
	sort.Slice(files, func(i, j int) bool {
		return files[i].Key < files[j].Key
	})

	// Apply limit
	var nextCursor string
	if len(files) > limit {
		files = files[:limit]
		nextCursor = files[limit-1].Key
	}

	return files, nextCursor, nil
}

// Copy duplicates a file from srcKey to dstKey.
func (b *LocalBackend) Copy(ctx context.Context, srcKey, dstKey string) error {
	if err := b.validateKey(srcKey); err != nil {
		return err
	}
	if err := b.validateKey(dstKey); err != nil {
		return err
	}

	srcPath := b.fullPath(srcKey)
	dstPath := b.fullPath(dstKey)

	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Error{Op: "Copy", Key: srcKey, Err: errNotFound{}}
		}
		return &Error{Op: "Copy", Key: srcKey, Err: err}
	}
	defer src.Close()

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return &Error{Op: "Copy", Key: dstKey, Err: fmt.Errorf("create directory: %w", err)}
	}

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return &Error{Op: "Copy", Key: dstKey, Err: err}
	}
	defer dst.Close()

	// Copy content
	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(dstPath)
		return &Error{Op: "Copy", Key: dstKey, Err: err}
	}

	return nil
}

// Close releases resources.
func (b *LocalBackend) Close() error {
	return nil // Local backend has no resources to release
}

// BasePath returns the base path of the local backend.
// Useful for debugging and testing.
func (b *LocalBackend) BasePath() string {
	return b.basePath
}
