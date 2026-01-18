// Package backend defines the storage backend interface for sblite file storage.
// Backends handle the actual file storage operations (local filesystem, S3, etc.)
// while the storage service handles metadata and access control.
package backend

import (
	"context"
	"io"
	"time"
)

// FileInfo represents metadata about a stored file.
type FileInfo struct {
	Key         string    // Full path/key of the file
	Size        int64     // File size in bytes
	ContentType string    // MIME type
	ETag        string    // MD5 hash for caching/integrity
	ModTime     time.Time // Last modification time
}

// Backend defines the interface for storage backends.
// Implementations must be safe for concurrent use.
type Backend interface {
	// Exists checks if a file exists at the given key.
	Exists(ctx context.Context, key string) (bool, error)

	// Attributes returns metadata for a file without reading its content.
	// Returns ErrNotFound if the file does not exist.
	Attributes(ctx context.Context, key string) (*FileInfo, error)

	// Reader returns a reader for the file content along with its metadata.
	// The caller is responsible for closing the reader.
	// Returns ErrNotFound if the file does not exist.
	Reader(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)

	// Write stores content at the given key.
	// If size is -1, the implementation should read until EOF.
	// The contentType should be a valid MIME type.
	// Returns metadata about the written file including its ETag.
	Write(ctx context.Context, key string, content io.Reader, size int64, contentType string) (*FileInfo, error)

	// Delete removes a file at the given key.
	// Returns nil if the file does not exist (idempotent).
	Delete(ctx context.Context, key string) error

	// DeletePrefix removes all files with the given prefix.
	// Returns slice of errors for files that failed to delete.
	// An empty prefix will delete all files (use with caution).
	DeletePrefix(ctx context.Context, prefix string) []error

	// List returns files with the given prefix.
	// Results are paginated: limit controls max results, cursor is the last key from previous call.
	// Returns files, next cursor (empty if no more results), and error.
	List(ctx context.Context, prefix string, limit int, cursor string) ([]FileInfo, string, error)

	// Copy duplicates a file from srcKey to dstKey.
	// Returns ErrNotFound if source does not exist.
	Copy(ctx context.Context, srcKey, dstKey string) error

	// Close releases any resources held by the backend.
	Close() error
}

// Error types for backend operations
type Error struct {
	Op   string // Operation that failed
	Key  string // Key involved
	Err  error  // Underlying error
}

func (e *Error) Error() string {
	if e.Key != "" {
		return e.Op + " " + e.Key + ": " + e.Err.Error()
	}
	return e.Op + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Sentinel errors
var (
	ErrNotFound      = &Error{Op: "storage", Err: errNotFound{}}
	ErrInvalidKey    = &Error{Op: "storage", Err: errInvalidKey{}}
	ErrAccessDenied  = &Error{Op: "storage", Err: errAccessDenied{}}
)

type errNotFound struct{}
func (errNotFound) Error() string { return "file not found" }

type errInvalidKey struct{}
func (errInvalidKey) Error() string { return "invalid key" }

type errAccessDenied struct{}
func (errAccessDenied) Error() string { return "access denied" }

// IsNotFound returns true if the error indicates a file was not found.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*errNotFound)
	if ok {
		return true
	}
	if e, ok := err.(*Error); ok {
		_, ok := e.Err.(errNotFound)
		return ok
	}
	return false
}

// IsInvalidKey returns true if the error indicates an invalid key.
func IsInvalidKey(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		_, ok := e.Err.(errInvalidKey)
		return ok
	}
	return false
}
