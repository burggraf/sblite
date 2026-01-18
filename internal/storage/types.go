// Package storage implements Supabase-compatible file storage for sblite.
// It provides bucket management, object storage, and access control.
package storage

import (
	"encoding/json"
	"time"
)

// Bucket represents a storage bucket.
// Matches Supabase storage.buckets schema.
type Bucket struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Owner            string   `json:"owner,omitempty"`
	OwnerID          string   `json:"owner_id,omitempty"`
	Public           bool     `json:"public"`
	FileSizeLimit    *int64   `json:"file_size_limit,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

// CreateBucketRequest is the request body for creating a bucket.
type CreateBucketRequest struct {
	ID               string   `json:"id,omitempty"`
	Name             string   `json:"name"`
	Public           bool     `json:"public,omitempty"`
	FileSizeLimit    *int64   `json:"file_size_limit,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
}

// UpdateBucketRequest is the request body for updating a bucket.
type UpdateBucketRequest struct {
	Public           *bool    `json:"public,omitempty"`
	FileSizeLimit    *int64   `json:"file_size_limit,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
}

// Object represents a stored file's metadata.
// Matches Supabase storage.objects schema.
type Object struct {
	ID             string            `json:"id"`
	BucketID       string            `json:"bucket_id"`
	Name           string            `json:"name"`
	Owner          string            `json:"owner,omitempty"`
	OwnerID        string            `json:"owner_id,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
	PathTokens     []string          `json:"path_tokens,omitempty"`
	UserMetadata   map[string]string `json:"user_metadata,omitempty"`
	Version        string            `json:"version,omitempty"`
	Size           int64             `json:"size,omitempty"`
	MimeType       string            `json:"mime_type,omitempty"`
	ETag           string            `json:"etag,omitempty"`
	LastAccessedAt *string           `json:"last_accessed_at,omitempty"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

// ListObjectsRequest is the request body for listing objects.
type ListObjectsRequest struct {
	Prefix string         `json:"prefix"`
	Limit  int            `json:"limit,omitempty"`
	Offset int            `json:"offset,omitempty"`
	Search string         `json:"search,omitempty"`
	SortBy *SortByOptions `json:"sortBy,omitempty"`
}

// SortByOptions specifies sorting for object listing.
type SortByOptions struct {
	Column string `json:"column"` // name, updated_at, created_at, last_accessed_at
	Order  string `json:"order"`  // asc, desc
}

// SignedURLRequest is the request body for creating a signed URL.
type SignedURLRequest struct {
	ExpiresIn int               `json:"expiresIn"` // seconds
	Transform *TransformOptions `json:"transform,omitempty"`
}

// SignedURLResponse is the response for creating a signed URL.
type SignedURLResponse struct {
	SignedURL string `json:"signedURL"`
}

// SignedURLsRequest is the request body for creating multiple signed URLs.
type SignedURLsRequest struct {
	ExpiresIn int      `json:"expiresIn"` // seconds
	Paths     []string `json:"paths"`
}

// SignedURLsResponseItem is a single item in the batch signed URLs response.
type SignedURLsResponseItem struct {
	Path      string  `json:"path"`
	SignedURL string  `json:"signedURL"`
	Error     *string `json:"error"`
}

// SignedUploadURLResponse is the response for creating a signed upload URL.
// Note: SDK expects "url" field (not "signedUrl") and extracts token from query params.
type SignedUploadURLResponse struct {
	URL string `json:"url"`
}

// UploadToSignedURLResponse is the response for uploading to a signed URL.
type UploadToSignedURLResponse struct {
	Key  string `json:"Key"`
	Path string `json:"path"`
}

// TransformOptions specifies image transformation parameters.
type TransformOptions struct {
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Resize  string `json:"resize,omitempty"`  // cover, contain, fill
	Format  string `json:"format,omitempty"`  // origin, webp, jpeg, png
	Quality int    `json:"quality,omitempty"` // 20-100
}

// CopyObjectRequest is the request body for copying an object.
type CopyObjectRequest struct {
	BucketID          string          `json:"bucketId"`
	SourceKey         string          `json:"sourceKey"`
	DestinationBucket string          `json:"destinationBucket,omitempty"`
	DestinationKey    string          `json:"destinationKey"`
	CopyMetadata      bool            `json:"copyMetadata,omitempty"`
	Metadata          *ObjectMetadata `json:"metadata,omitempty"`
}

// MoveObjectRequest is the request body for moving an object.
type MoveObjectRequest struct {
	BucketID          string `json:"bucketId"`
	SourceKey         string `json:"sourceKey"`
	DestinationBucket string `json:"destinationBucket,omitempty"`
	DestinationKey    string `json:"destinationKey"`
}

// ObjectMetadata contains optional metadata for objects.
type ObjectMetadata struct {
	CacheControl string `json:"cacheControl,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	MimeType     string `json:"mimetype,omitempty"`
}

// UploadResponse is the response for uploading an object.
type UploadResponse struct {
	ID  string `json:"Id,omitempty"`
	Key string `json:"Key"`
}

// DeleteResponse is the response for deleting an object.
type DeleteResponse struct {
	Message string `json:"message"`
}

// DeletedObject represents a deleted object in batch delete response.
type DeletedObject struct {
	BucketID string `json:"bucket_id"`
	Name     string `json:"name"`
}

// StorageError represents an error from storage operations.
type StorageError struct {
	StatusCode int    `json:"statusCode"`
	ErrorCode  string `json:"error"`
	Message    string `json:"message"`
}

func (e *StorageError) Error() string {
	return e.Message
}

// Helper functions

// Now returns current time in ISO 8601 format.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ParsePathTokens splits a path into tokens for RLS.
func ParsePathTokens(path string) []string {
	if path == "" {
		return []string{}
	}
	var tokens []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}

// MarshalJSON helper for slice to JSON string
func MarshalJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
