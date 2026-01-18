package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// UploadObject uploads a file to a bucket.
func (s *Service) UploadObject(bucketName, objectPath string, content io.Reader, size int64, contentType string, ownerID string, upsert bool) (*UploadResponse, error) {
	// Get bucket
	bucket, err := s.GetBucketByName(bucketName)
	if err != nil {
		return nil, err
	}

	// Validate file size limit
	if bucket.FileSizeLimit != nil && size > *bucket.FileSizeLimit {
		return nil, &StorageError{
			StatusCode: 413,
			ErrorCode:  "payload_too_large",
			Message:    fmt.Sprintf("File size exceeds limit of %d bytes", *bucket.FileSizeLimit),
		}
	}

	// Validate MIME type
	if len(bucket.AllowedMimeTypes) > 0 {
		allowed := false
		for _, mt := range bucket.AllowedMimeTypes {
			if mt == contentType || strings.HasPrefix(contentType, strings.TrimSuffix(mt, "*")) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, &StorageError{
				StatusCode: 415,
				ErrorCode:  "invalid_mime_type",
				Message:    fmt.Sprintf("MIME type %s is not allowed", contentType),
			}
		}
	}

	// Sanitize object path
	objectPath = strings.TrimPrefix(objectPath, "/")
	if objectPath == "" {
		return nil, &StorageError{StatusCode: 400, ErrorCode: "invalid_path", Message: "Object path is required"}
	}

	// Check if object exists
	var existingID string
	err = s.db.QueryRow("SELECT id FROM storage_objects WHERE bucket_id = ? AND name = ?", bucket.ID, objectPath).Scan(&existingID)
	exists := err == nil

	if exists && !upsert {
		return nil, &StorageError{StatusCode: 400, ErrorCode: "object_exists", Message: "Object already exists. Use upsert to overwrite."}
	}

	// Storage key: bucket_id/object_path
	storageKey := bucket.ID + "/" + objectPath

	// Write to backend
	fileInfo, err := s.backend.Write(s.ctx, storageKey, content, size, contentType)
	if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to write file: %v", err)}
	}

	now := Now()
	pathTokens := ParsePathTokens(objectPath)
	pathTokensJSON := MarshalJSONString(pathTokens)

	if exists {
		// Update existing object
		_, err = s.db.Exec(`
			UPDATE storage_objects
			SET size = ?, mime_type = ?, etag = ?, updated_at = ?, path_tokens = ?
			WHERE id = ?
		`, fileInfo.Size, contentType, fileInfo.ETag, now, pathTokensJSON, existingID)
		if err != nil {
			return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to update object: %v", err)}
		}

		return &UploadResponse{
			ID:  existingID,
			Key: bucketName + "/" + objectPath,
		}, nil
	}

	// Create new object
	id := generateUUID()
	_, err = s.db.Exec(`
		INSERT INTO storage_objects (id, bucket_id, name, owner_id, size, mime_type, etag, path_tokens, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, bucket.ID, objectPath, nilIfEmpty(ownerID), fileInfo.Size, contentType, fileInfo.ETag, pathTokensJSON, now, now)
	if err != nil {
		// Cleanup file on db error
		s.backend.Delete(s.ctx, storageKey)
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to create object: %v", err)}
	}

	return &UploadResponse{
		ID:  id,
		Key: bucketName + "/" + objectPath,
	}, nil
}

// GetObject retrieves a file from a bucket.
// Returns the reader, content type, and size.
func (s *Service) GetObject(bucketName, objectPath string) (io.ReadCloser, string, int64, error) {
	// Get bucket
	bucket, err := s.GetBucketByName(bucketName)
	if err != nil {
		return nil, "", 0, err
	}

	objectPath = strings.TrimPrefix(objectPath, "/")

	// Get object metadata
	var obj Object
	var mimeType sql.NullString
	var size sql.NullInt64

	err = s.db.QueryRow(`
		SELECT id, mime_type, size FROM storage_objects WHERE bucket_id = ? AND name = ?
	`, bucket.ID, objectPath).Scan(&obj.ID, &mimeType, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", 0, &StorageError{StatusCode: 404, ErrorCode: "not_found", Message: "Object not found"}
	} else if err != nil {
		return nil, "", 0, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to get object: %v", err)}
	}

	// Get from backend
	storageKey := bucket.ID + "/" + objectPath
	reader, _, err := s.backend.Reader(s.ctx, storageKey)
	if err != nil {
		return nil, "", 0, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to read file: %v", err)}
	}

	// Update last_accessed_at
	s.db.Exec("UPDATE storage_objects SET last_accessed_at = ? WHERE id = ?", Now(), obj.ID)

	contentType := mimeType.String
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return reader, contentType, size.Int64, nil
}

// GetObjectInfo retrieves object metadata without content.
func (s *Service) GetObjectInfo(bucketName, objectPath string) (*Object, error) {
	// Get bucket
	bucket, err := s.GetBucketByName(bucketName)
	if err != nil {
		return nil, err
	}

	objectPath = strings.TrimPrefix(objectPath, "/")

	var obj Object
	var mimeType, etag, owner, ownerID, version, lastAccessed sql.NullString
	var size sql.NullInt64
	var metadata, pathTokens, userMetadata sql.NullString

	err = s.db.QueryRow(`
		SELECT id, bucket_id, name, owner, owner_id, metadata, path_tokens, user_metadata,
		       version, size, mime_type, etag, last_accessed_at, created_at, updated_at
		FROM storage_objects WHERE bucket_id = ? AND name = ?
	`, bucket.ID, objectPath).Scan(
		&obj.ID, &obj.BucketID, &obj.Name, &owner, &ownerID, &metadata, &pathTokens, &userMetadata,
		&version, &size, &mimeType, &etag, &lastAccessed, &obj.CreatedAt, &obj.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &StorageError{StatusCode: 404, ErrorCode: "not_found", Message: "Object not found"}
	} else if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to get object: %v", err)}
	}

	obj.Owner = owner.String
	obj.OwnerID = ownerID.String
	obj.Version = version.String
	obj.Size = size.Int64
	obj.MimeType = mimeType.String
	obj.ETag = etag.String
	if lastAccessed.Valid {
		obj.LastAccessedAt = &lastAccessed.String
	}

	if metadata.Valid {
		json.Unmarshal([]byte(metadata.String), &obj.Metadata)
	}
	if pathTokens.Valid {
		json.Unmarshal([]byte(pathTokens.String), &obj.PathTokens)
	}
	if userMetadata.Valid {
		json.Unmarshal([]byte(userMetadata.String), &obj.UserMetadata)
	}

	return &obj, nil
}

// DeleteObject deletes a file from a bucket.
func (s *Service) DeleteObject(bucketName, objectPath string) error {
	// Get bucket
	bucket, err := s.GetBucketByName(bucketName)
	if err != nil {
		return err
	}

	objectPath = strings.TrimPrefix(objectPath, "/")

	// Check object exists
	var id string
	err = s.db.QueryRow("SELECT id FROM storage_objects WHERE bucket_id = ? AND name = ?", bucket.ID, objectPath).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return &StorageError{StatusCode: 404, ErrorCode: "not_found", Message: "Object not found"}
	} else if err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to get object: %v", err)}
	}

	// Delete from backend
	storageKey := bucket.ID + "/" + objectPath
	if err := s.backend.Delete(s.ctx, storageKey); err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to delete file: %v", err)}
	}

	// Delete from database
	_, err = s.db.Exec("DELETE FROM storage_objects WHERE id = ?", id)
	if err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to delete object: %v", err)}
	}

	return nil
}

// DeleteObjects deletes multiple files from a bucket.
func (s *Service) DeleteObjects(bucketName string, paths []string) []error {
	var errors []error
	for _, path := range paths {
		if err := s.DeleteObject(bucketName, path); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// ListObjects lists objects in a bucket with a prefix.
func (s *Service) ListObjects(bucketName string, req ListObjectsRequest) ([]Object, error) {
	// Get bucket
	bucket, err := s.GetBucketByName(bucketName)
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	// Build query
	query := `
		SELECT id, bucket_id, name, owner, owner_id, metadata, path_tokens, user_metadata,
		       version, size, mime_type, etag, last_accessed_at, created_at, updated_at
		FROM storage_objects
		WHERE bucket_id = ?
	`
	args := []any{bucket.ID}

	if req.Prefix != "" {
		query += " AND name LIKE ?"
		args = append(args, req.Prefix+"%")
	}

	if req.Search != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+req.Search+"%")
	}

	// Sort
	sortColumn := "name"
	sortOrder := "ASC"
	if req.SortBy != nil {
		switch req.SortBy.Column {
		case "name", "updated_at", "created_at", "last_accessed_at":
			sortColumn = req.SortBy.Column
		}
		if strings.ToUpper(req.SortBy.Order) == "DESC" {
			sortOrder = "DESC"
		}
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortColumn, sortOrder)

	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, req.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to list objects: %v", err)}
	}
	defer rows.Close()

	var objects []Object
	for rows.Next() {
		var obj Object
		var mimeType, etag, owner, ownerID, version, lastAccessed sql.NullString
		var size sql.NullInt64
		var metadata, pathTokens, userMetadata sql.NullString

		err := rows.Scan(
			&obj.ID, &obj.BucketID, &obj.Name, &owner, &ownerID, &metadata, &pathTokens, &userMetadata,
			&version, &size, &mimeType, &etag, &lastAccessed, &obj.CreatedAt, &obj.UpdatedAt,
		)
		if err != nil {
			return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to scan object: %v", err)}
		}

		obj.Owner = owner.String
		obj.OwnerID = ownerID.String
		obj.Version = version.String
		obj.Size = size.Int64
		obj.MimeType = mimeType.String
		obj.ETag = etag.String
		if lastAccessed.Valid {
			obj.LastAccessedAt = &lastAccessed.String
		}

		if metadata.Valid {
			json.Unmarshal([]byte(metadata.String), &obj.Metadata)
		}
		if pathTokens.Valid {
			json.Unmarshal([]byte(pathTokens.String), &obj.PathTokens)
		}
		if userMetadata.Valid {
			json.Unmarshal([]byte(userMetadata.String), &obj.UserMetadata)
		}

		// Strip prefix from name when listing (Supabase API returns relative names)
		if req.Prefix != "" && strings.HasPrefix(obj.Name, req.Prefix) {
			obj.Name = strings.TrimPrefix(obj.Name, req.Prefix)
			obj.Name = strings.TrimPrefix(obj.Name, "/")
		}

		objects = append(objects, obj)
	}

	if objects == nil {
		objects = []Object{}
	}

	return objects, nil
}

// CopyObject copies an object within or between buckets.
func (s *Service) CopyObject(req CopyObjectRequest, ownerID string) (*UploadResponse, error) {
	// Get source bucket
	srcBucket, err := s.GetBucket(req.BucketID)
	if err != nil {
		return nil, err
	}

	// Get destination bucket (default to source)
	dstBucketID := req.DestinationBucket
	if dstBucketID == "" {
		dstBucketID = req.BucketID
	}
	dstBucket, err := s.GetBucket(dstBucketID)
	if err != nil {
		return nil, err
	}

	// Check source object exists
	srcObj, err := s.GetObjectInfo(srcBucket.Name, req.SourceKey)
	if err != nil {
		return nil, err
	}

	// Storage keys
	srcKey := srcBucket.ID + "/" + req.SourceKey
	dstKey := dstBucket.ID + "/" + req.DestinationKey

	// Copy in backend
	if err := s.backend.Copy(s.ctx, srcKey, dstKey); err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to copy file: %v", err)}
	}

	now := Now()
	id := generateUUID()
	pathTokens := ParsePathTokens(req.DestinationKey)
	pathTokensJSON := MarshalJSONString(pathTokens)

	// Get content type
	mimeType := srcObj.MimeType
	if req.Metadata != nil && req.Metadata.MimeType != "" {
		mimeType = req.Metadata.MimeType
	} else if req.Metadata != nil && req.Metadata.ContentType != "" {
		mimeType = req.Metadata.ContentType
	}

	// Create destination object record
	_, err = s.db.Exec(`
		INSERT INTO storage_objects (id, bucket_id, name, owner_id, size, mime_type, etag, path_tokens, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_id, name) DO UPDATE SET
		    size = excluded.size, mime_type = excluded.mime_type, etag = excluded.etag,
		    path_tokens = excluded.path_tokens, updated_at = excluded.updated_at
	`, id, dstBucket.ID, req.DestinationKey, nilIfEmpty(ownerID), srcObj.Size, mimeType, srcObj.ETag, pathTokensJSON, now, now)
	if err != nil {
		s.backend.Delete(s.ctx, dstKey)
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to create object: %v", err)}
	}

	return &UploadResponse{
		ID:  id,
		Key: dstBucket.Name + "/" + req.DestinationKey,
	}, nil
}

// MoveObject moves an object within or between buckets.
func (s *Service) MoveObject(req MoveObjectRequest, ownerID string) error {
	// Copy first
	_, err := s.CopyObject(CopyObjectRequest{
		BucketID:          req.BucketID,
		SourceKey:         req.SourceKey,
		DestinationBucket: req.DestinationBucket,
		DestinationKey:    req.DestinationKey,
		CopyMetadata:      true,
	}, ownerID)
	if err != nil {
		return err
	}

	// Get source bucket
	srcBucket, err := s.GetBucket(req.BucketID)
	if err != nil {
		return err
	}

	// Delete source
	return s.DeleteObject(srcBucket.Name, req.SourceKey)
}

// DetectContentType detects the content type of a file.
func DetectContentType(filename string, content []byte) string {
	// Try to detect from content first (first 512 bytes)
	if len(content) > 0 {
		contentType := http.DetectContentType(content)
		if contentType != "application/octet-stream" {
			return contentType
		}
	}

	// Fall back to extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".gz", ".gzip":
		return "application/gzip"
	case ".tar":
		return "application/x-tar"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	default:
		return "application/octet-stream"
	}
}

// IsBucketPublic checks if a bucket is public.
func (s *Service) IsBucketPublic(bucketName string) (bool, error) {
	bucket, err := s.GetBucketByName(bucketName)
	if err != nil {
		return false, err
	}
	return bucket.Public, nil
}

// Context returns the service context.
func (s *Service) Context() context.Context {
	return s.ctx
}
