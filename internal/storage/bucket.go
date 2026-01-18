package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateBucket creates a new storage bucket.
func (s *Service) CreateBucket(req CreateBucketRequest, ownerID string) (*Bucket, error) {
	// Generate ID if not provided
	id := req.ID
	if id == "" {
		id = req.Name // Supabase uses name as ID by default
	}

	// Validate name
	if req.Name == "" {
		return nil, &StorageError{StatusCode: 400, ErrorCode: "invalid_name", Message: "Bucket name is required"}
	}

	// Check if bucket already exists
	var exists int
	err := s.db.QueryRow("SELECT 1 FROM storage_buckets WHERE id = ? OR name = ?", id, req.Name).Scan(&exists)
	if err == nil {
		return nil, &StorageError{StatusCode: 400, ErrorCode: "bucket_exists", Message: "Bucket already exists"}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to check bucket: %v", err)}
	}

	now := Now()

	// Serialize allowed_mime_types
	var mimeTypesJSON *string
	if len(req.AllowedMimeTypes) > 0 {
		b, _ := json.Marshal(req.AllowedMimeTypes)
		s := string(b)
		mimeTypesJSON = &s
	}

	// Insert bucket
	_, err = s.db.Exec(`
		INSERT INTO storage_buckets (id, name, owner_id, public, file_size_limit, allowed_mime_types, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.Name, nilIfEmpty(ownerID), boolToInt(req.Public), req.FileSizeLimit, mimeTypesJSON, now, now)
	if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to create bucket: %v", err)}
	}

	return &Bucket{
		ID:               id,
		Name:             req.Name,
		OwnerID:          ownerID,
		Public:           req.Public,
		FileSizeLimit:    req.FileSizeLimit,
		AllowedMimeTypes: req.AllowedMimeTypes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// GetBucket retrieves a bucket by ID.
func (s *Service) GetBucket(id string) (*Bucket, error) {
	var bucket Bucket
	var public int
	var fileSizeLimit sql.NullInt64
	var mimeTypesJSON sql.NullString
	var owner, ownerID sql.NullString

	err := s.db.QueryRow(`
		SELECT id, name, owner, owner_id, public, file_size_limit, allowed_mime_types, created_at, updated_at
		FROM storage_buckets WHERE id = ?
	`, id).Scan(&bucket.ID, &bucket.Name, &owner, &ownerID, &public, &fileSizeLimit, &mimeTypesJSON, &bucket.CreatedAt, &bucket.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &StorageError{StatusCode: 404, ErrorCode: "not_found", Message: "Bucket not found"}
	} else if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to get bucket: %v", err)}
	}

	bucket.Public = public == 1
	bucket.Owner = owner.String
	bucket.OwnerID = ownerID.String

	if fileSizeLimit.Valid {
		bucket.FileSizeLimit = &fileSizeLimit.Int64
	}

	if mimeTypesJSON.Valid {
		json.Unmarshal([]byte(mimeTypesJSON.String), &bucket.AllowedMimeTypes)
	}

	return &bucket, nil
}

// GetBucketByName retrieves a bucket by name.
func (s *Service) GetBucketByName(name string) (*Bucket, error) {
	var bucket Bucket
	var public int
	var fileSizeLimit sql.NullInt64
	var mimeTypesJSON sql.NullString
	var owner, ownerID sql.NullString

	err := s.db.QueryRow(`
		SELECT id, name, owner, owner_id, public, file_size_limit, allowed_mime_types, created_at, updated_at
		FROM storage_buckets WHERE name = ?
	`, name).Scan(&bucket.ID, &bucket.Name, &owner, &ownerID, &public, &fileSizeLimit, &mimeTypesJSON, &bucket.CreatedAt, &bucket.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &StorageError{StatusCode: 404, ErrorCode: "not_found", Message: "Bucket not found"}
	} else if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to get bucket: %v", err)}
	}

	bucket.Public = public == 1
	bucket.Owner = owner.String
	bucket.OwnerID = ownerID.String

	if fileSizeLimit.Valid {
		bucket.FileSizeLimit = &fileSizeLimit.Int64
	}

	if mimeTypesJSON.Valid {
		json.Unmarshal([]byte(mimeTypesJSON.String), &bucket.AllowedMimeTypes)
	}

	return &bucket, nil
}

// ListBuckets returns all buckets.
func (s *Service) ListBuckets(limit, offset int, search string) ([]Bucket, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, name, owner, owner_id, public, file_size_limit, allowed_mime_types, created_at, updated_at FROM storage_buckets`
	args := []any{}

	if search != "" {
		query += " WHERE name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	query += " ORDER BY name ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to list buckets: %v", err)}
	}
	defer rows.Close()

	var buckets []Bucket
	for rows.Next() {
		var bucket Bucket
		var public int
		var fileSizeLimit sql.NullInt64
		var mimeTypesJSON sql.NullString
		var owner, ownerID sql.NullString

		err := rows.Scan(&bucket.ID, &bucket.Name, &owner, &ownerID, &public, &fileSizeLimit, &mimeTypesJSON, &bucket.CreatedAt, &bucket.UpdatedAt)
		if err != nil {
			return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to scan bucket: %v", err)}
		}

		bucket.Public = public == 1
		bucket.Owner = owner.String
		bucket.OwnerID = ownerID.String

		if fileSizeLimit.Valid {
			bucket.FileSizeLimit = &fileSizeLimit.Int64
		}

		if mimeTypesJSON.Valid {
			json.Unmarshal([]byte(mimeTypesJSON.String), &bucket.AllowedMimeTypes)
		}

		buckets = append(buckets, bucket)
	}

	if buckets == nil {
		buckets = []Bucket{}
	}

	return buckets, nil
}

// UpdateBucket updates a bucket's configuration.
func (s *Service) UpdateBucket(id string, req UpdateBucketRequest) (*Bucket, error) {
	// Check bucket exists
	bucket, err := s.GetBucket(id)
	if err != nil {
		return nil, err
	}

	now := Now()

	// Build update query
	updates := []string{"updated_at = ?"}
	args := []any{now}

	if req.Public != nil {
		updates = append(updates, "public = ?")
		args = append(args, boolToInt(*req.Public))
		bucket.Public = *req.Public
	}

	if req.FileSizeLimit != nil {
		updates = append(updates, "file_size_limit = ?")
		args = append(args, *req.FileSizeLimit)
		bucket.FileSizeLimit = req.FileSizeLimit
	}

	if req.AllowedMimeTypes != nil {
		var mimeTypesJSON *string
		if len(req.AllowedMimeTypes) > 0 {
			b, _ := json.Marshal(req.AllowedMimeTypes)
			s := string(b)
			mimeTypesJSON = &s
		}
		updates = append(updates, "allowed_mime_types = ?")
		args = append(args, mimeTypesJSON)
		bucket.AllowedMimeTypes = req.AllowedMimeTypes
	}

	args = append(args, id)

	query := fmt.Sprintf("UPDATE storage_buckets SET %s WHERE id = ?", joinStrings(updates, ", "))
	_, err = s.db.Exec(query, args...)
	if err != nil {
		return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to update bucket: %v", err)}
	}

	bucket.UpdatedAt = now
	return bucket, nil
}

// DeleteBucket deletes a bucket.
// The bucket must be empty or force must be true.
func (s *Service) DeleteBucket(id string, force bool) error {
	// Check bucket exists
	_, err := s.GetBucket(id)
	if err != nil {
		return err
	}

	// Check if bucket is empty
	if !force {
		var count int
		err := s.db.QueryRow("SELECT COUNT(*) FROM storage_objects WHERE bucket_id = ?", id).Scan(&count)
		if err != nil {
			return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to check bucket: %v", err)}
		}
		if count > 0 {
			return &StorageError{StatusCode: 400, ErrorCode: "bucket_not_empty", Message: "Bucket is not empty"}
		}
	}

	// Delete objects from backend
	if force {
		s.backend.DeletePrefix(s.ctx, id+"/")
	}

	// Delete bucket and objects (cascade)
	_, err = s.db.Exec("DELETE FROM storage_buckets WHERE id = ?", id)
	if err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to delete bucket: %v", err)}
	}

	return nil
}

// EmptyBucket removes all objects from a bucket.
func (s *Service) EmptyBucket(id string) error {
	// Check bucket exists
	_, err := s.GetBucket(id)
	if err != nil {
		return err
	}

	// Delete all files from backend
	s.backend.DeletePrefix(s.ctx, id+"/")

	// Delete all object records
	_, err = s.db.Exec("DELETE FROM storage_objects WHERE bucket_id = ?", id)
	if err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: fmt.Sprintf("Failed to empty bucket: %v", err)}
	}

	return nil
}

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// generateUUID generates a new UUID v4.
func generateUUID() string {
	return uuid.New().String()
}
