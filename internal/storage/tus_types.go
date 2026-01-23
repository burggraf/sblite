package storage

import "time"

// TUS protocol version
const TUSVersion = "1.0.0"

// TUS extensions supported
const TUSExtensions = "creation,creation-with-upload,termination"

// TUSConfig holds configuration for the TUS service.
type TUSConfig struct {
	// MaxSize is the maximum upload size in bytes (0 = unlimited)
	MaxSize int64

	// ChunkSize is the recommended chunk size for clients (advisory only)
	ChunkSize int64

	// ExpirationDuration is how long an upload session remains valid
	ExpirationDuration time.Duration
}

// DefaultTUSConfig returns default TUS configuration.
func DefaultTUSConfig() TUSConfig {
	return TUSConfig{
		MaxSize:            0, // Unlimited by default (bucket limits still apply)
		ChunkSize:          5 * 1024 * 1024, // 5 MB recommended chunks
		ExpirationDuration: 24 * time.Hour,
	}
}

// UploadSession represents an in-progress TUS upload.
type UploadSession struct {
	ID           string `json:"id"`
	BucketID     string `json:"bucket_id"`
	ObjectName   string `json:"object_name"`
	OwnerID      string `json:"owner_id,omitempty"`
	UploadLength int64  `json:"upload_length"`
	UploadOffset int64  `json:"upload_offset"`
	ContentType  string `json:"content_type,omitempty"`
	CacheControl string `json:"cache_control,omitempty"`
	Metadata     string `json:"metadata,omitempty"` // JSON string
	Upsert       bool   `json:"upsert"`
	TempPath     string `json:"temp_path,omitempty"`     // For local backend
	S3UploadID   string `json:"s3_upload_id,omitempty"`  // For S3 multipart (future)
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
}

// IsComplete returns true if the upload has received all data.
func (s *UploadSession) IsComplete() bool {
	return s.UploadOffset >= s.UploadLength
}

// IsExpired returns true if the upload session has expired.
func (s *UploadSession) IsExpired() bool {
	expires, err := time.Parse(time.RFC3339, s.ExpiresAt)
	if err != nil {
		return true // Treat parse errors as expired
	}
	return time.Now().UTC().After(expires)
}

// CreateUploadRequest is the request to create a TUS upload session.
type CreateUploadRequest struct {
	BucketID     string
	ObjectName   string
	UploadLength int64
	ContentType  string
	CacheControl string
	Metadata     map[string]string // Parsed from Upload-Metadata header
	OwnerID      string
	Upsert       bool
}

// TUSError represents an error from TUS operations.
type TUSError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"error"`
}

func (e *TUSError) Error() string {
	return e.Message
}

// Common TUS errors
var (
	ErrTUSVersionMismatch = &TUSError{StatusCode: 412, Message: "Tus-Resumable header must be 1.0.0"}
	ErrTUSSessionNotFound = &TUSError{StatusCode: 404, Message: "Upload session not found"}
	ErrTUSSessionExpired  = &TUSError{StatusCode: 410, Message: "Upload session has expired"}
	ErrTUSOffsetMismatch  = &TUSError{StatusCode: 409, Message: "Upload-Offset does not match current offset"}
	ErrTUSInvalidLength   = &TUSError{StatusCode: 400, Message: "Invalid Upload-Length"}
	ErrTUSMissingMetadata = &TUSError{StatusCode: 400, Message: "Missing required metadata: bucketName and objectName"}
)
