package storage

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/markb/sblite/internal/log"
	"github.com/markb/sblite/internal/storage/backend"
)

// TUSService handles TUS resumable upload operations.
type TUSService struct {
	db        *sql.DB
	backend   backend.Backend
	config    TUSConfig
	uploadsDir string // Directory for temporary upload files
	mu        sync.Mutex
}

// NewTUSService creates a new TUS service.
func NewTUSService(db *sql.DB, b backend.Backend, cfg TUSConfig, uploadsDir string) *TUSService {
	// Ensure uploads directory exists
	if uploadsDir != "" {
		os.MkdirAll(uploadsDir, 0755)
	}

	return &TUSService{
		db:         db,
		backend:    b,
		config:     cfg,
		uploadsDir: uploadsDir,
	}
}

// CreateUpload creates a new upload session.
func (s *TUSService) CreateUpload(ctx context.Context, req CreateUploadRequest) (*UploadSession, error) {
	// Validate upload length
	if req.UploadLength < 0 {
		return nil, ErrTUSInvalidLength
	}

	// Check max size limit
	if s.config.MaxSize > 0 && req.UploadLength > s.config.MaxSize {
		return nil, &TUSError{
			StatusCode: 413,
			Message:    fmt.Sprintf("Upload size %d exceeds maximum allowed %d", req.UploadLength, s.config.MaxSize),
		}
	}

	id := generateUUID()
	now := time.Now().UTC()
	expiresAt := now.Add(s.config.ExpirationDuration)

	// Create temp file path
	tempPath := filepath.Join(s.uploadsDir, id+".tmp")

	session := &UploadSession{
		ID:           id,
		BucketID:     req.BucketID,
		ObjectName:   req.ObjectName,
		OwnerID:      req.OwnerID,
		UploadLength: req.UploadLength,
		UploadOffset: 0,
		ContentType:  req.ContentType,
		CacheControl: req.CacheControl,
		Metadata:     MarshalJSONString(req.Metadata),
		Upsert:       req.Upsert,
		TempPath:     tempPath,
		CreatedAt:    now.Format(time.RFC3339),
		ExpiresAt:    expiresAt.Format(time.RFC3339),
	}

	// Insert into database
	upsertInt := 0
	if req.Upsert {
		upsertInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO _resumable_uploads (
			id, bucket_id, object_name, owner_id, upload_length, upload_offset,
			content_type, cache_control, metadata, upsert, temp_path, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.BucketID, req.ObjectName, nilIfEmpty(req.OwnerID), req.UploadLength, 0,
		nilIfEmpty(req.ContentType), nilIfEmpty(req.CacheControl), session.Metadata, upsertInt,
		tempPath, session.CreatedAt, session.ExpiresAt)

	if err != nil {
		return nil, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to create upload session: %v", err)}
	}

	// Create empty temp file
	f, err := os.Create(tempPath)
	if err != nil {
		// Clean up database entry
		s.db.ExecContext(ctx, "DELETE FROM _resumable_uploads WHERE id = ?", id)
		return nil, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to create temp file: %v", err)}
	}
	f.Close()

	return session, nil
}

// GetUpload retrieves an upload session by ID.
func (s *TUSService) GetUpload(ctx context.Context, uploadID string) (*UploadSession, error) {
	session := &UploadSession{}
	var ownerID, contentType, cacheControl, metadata, tempPath, s3UploadID sql.NullString
	var upsert int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, bucket_id, object_name, owner_id, upload_length, upload_offset,
		       content_type, cache_control, metadata, upsert, temp_path, s3_upload_id,
		       created_at, expires_at
		FROM _resumable_uploads WHERE id = ?
	`, uploadID).Scan(
		&session.ID, &session.BucketID, &session.ObjectName, &ownerID,
		&session.UploadLength, &session.UploadOffset, &contentType, &cacheControl,
		&metadata, &upsert, &tempPath, &s3UploadID, &session.CreatedAt, &session.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTUSSessionNotFound
	}
	if err != nil {
		return nil, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to get upload session: %v", err)}
	}

	session.OwnerID = ownerID.String
	session.ContentType = contentType.String
	session.CacheControl = cacheControl.String
	session.Metadata = metadata.String
	session.Upsert = upsert == 1
	session.TempPath = tempPath.String
	session.S3UploadID = s3UploadID.String

	// Check expiration
	if session.IsExpired() {
		// Clean up expired session
		go s.deleteSession(context.Background(), session)
		return nil, ErrTUSSessionExpired
	}

	return session, nil
}

// WriteChunk writes a chunk of data to an upload session.
// Returns the new offset after writing.
func (s *TUSService) WriteChunk(ctx context.Context, uploadID string, offset int64, data io.Reader) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get session
	session, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return 0, err
	}

	// Verify offset matches
	if offset != session.UploadOffset {
		return session.UploadOffset, ErrTUSOffsetMismatch
	}

	// Open temp file and seek to offset
	f, err := os.OpenFile(session.TempPath, os.O_WRONLY, 0644)
	if err != nil {
		return session.UploadOffset, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to open temp file: %v", err)}
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return session.UploadOffset, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to seek in temp file: %v", err)}
	}

	// Calculate how much we can write
	remaining := session.UploadLength - offset
	limitedReader := io.LimitReader(data, remaining)

	// Write chunk
	written, err := io.Copy(f, limitedReader)
	if err != nil {
		return session.UploadOffset, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to write chunk: %v", err)}
	}

	// Update offset in database
	newOffset := offset + written
	_, err = s.db.ExecContext(ctx, "UPDATE _resumable_uploads SET upload_offset = ? WHERE id = ?", newOffset, uploadID)
	if err != nil {
		return session.UploadOffset, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to update offset: %v", err)}
	}

	return newOffset, nil
}

// FinalizeUpload completes an upload and moves it to the final location.
// Returns the finalized object key.
func (s *TUSService) FinalizeUpload(ctx context.Context, uploadID string, storageSvc *Service) (*UploadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get session
	session, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return nil, err
	}

	// Verify upload is complete
	if !session.IsComplete() {
		return nil, &TUSError{
			StatusCode: 400,
			Message:    fmt.Sprintf("Upload not complete: %d/%d bytes", session.UploadOffset, session.UploadLength),
		}
	}

	// Open the completed temp file
	f, err := os.Open(session.TempPath)
	if err != nil {
		return nil, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to open temp file: %v", err)}
	}
	defer f.Close()

	// Get bucket name
	var bucketName string
	err = s.db.QueryRowContext(ctx, "SELECT name FROM storage_buckets WHERE id = ?", session.BucketID).Scan(&bucketName)
	if err != nil {
		return nil, &TUSError{StatusCode: 500, Message: fmt.Sprintf("Failed to get bucket: %v", err)}
	}

	// Determine content type
	contentType := session.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to final location using storage service
	resp, err := storageSvc.UploadObject(
		bucketName,
		session.ObjectName,
		f,
		session.UploadLength,
		contentType,
		session.OwnerID,
		session.Upsert,
	)
	if err != nil {
		return nil, err
	}

	// Clean up session and temp file
	s.deleteSession(ctx, session)

	return resp, nil
}

// CancelUpload cancels an upload and cleans up resources.
func (s *TUSService) CancelUpload(ctx context.Context, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return err
	}

	return s.deleteSession(ctx, session)
}

// deleteSession removes a session and its temp file.
func (s *TUSService) deleteSession(ctx context.Context, session *UploadSession) error {
	// Delete temp file
	if session.TempPath != "" {
		os.Remove(session.TempPath)
	}

	// Delete database record
	_, err := s.db.ExecContext(ctx, "DELETE FROM _resumable_uploads WHERE id = ?", session.ID)
	return err
}

// CleanupExpired removes all expired upload sessions.
// Returns the number of sessions cleaned up.
func (s *TUSService) CleanupExpired(ctx context.Context) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Get expired sessions
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, temp_path FROM _resumable_uploads WHERE expires_at < ?
	`, now)
	if err != nil {
		return 0, fmt.Errorf("failed to query expired sessions: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id, tempPath string
		if err := rows.Scan(&id, &tempPath); err != nil {
			continue
		}

		// Delete temp file
		if tempPath != "" {
			os.Remove(tempPath)
		}

		// Delete record
		s.db.ExecContext(ctx, "DELETE FROM _resumable_uploads WHERE id = ?", id)
		count++
	}

	if count > 0 {
		log.Info("cleaned up expired TUS uploads", "count", count)
	}

	return count, nil
}

// StartCleanupRoutine starts a background routine to clean up expired sessions.
func (s *TUSService) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.CleanupExpired(ctx)
			}
		}
	}()
}

// CalculateETag calculates the MD5 ETag for a file.
func CalculateETag(path string) (string, error) {
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

