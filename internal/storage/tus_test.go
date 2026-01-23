package storage

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestParseMetadata(t *testing.T) {
	h := &TUSHandler{}

	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "empty",
			input: "",
			expected: map[string]string{},
		},
		{
			name:  "single pair",
			input: "bucketName dGVzdA==",
			expected: map[string]string{
				"bucketName": "test",
			},
		},
		{
			name:  "multiple pairs",
			input: "bucketName dGVzdA==,objectName ZmlsZS50eHQ=,contentType dGV4dC9wbGFpbg==",
			expected: map[string]string{
				"bucketName":  "test",
				"objectName":  "file.txt",
				"contentType": "text/plain",
			},
		},
		{
			name:  "with spaces",
			input: " bucketName dGVzdA== , objectName ZmlsZS50eHQ= ",
			expected: map[string]string{
				"bucketName": "test",
				"objectName": "file.txt",
			},
		},
		{
			name:  "invalid base64",
			input: "bucketName !!!invalid",
			expected: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := h.parseMetadata(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("expected %d pairs, got %d", len(tc.expected), len(result))
			}
			for k, v := range tc.expected {
				if result[k] != v {
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

func TestUploadSession_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		offset   int64
		length   int64
		expected bool
	}{
		{"not started", 0, 100, false},
		{"in progress", 50, 100, false},
		{"almost complete", 99, 100, false},
		{"exactly complete", 100, 100, true},
		{"over complete", 101, 100, true},
		{"zero length", 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &UploadSession{
				UploadOffset: tc.offset,
				UploadLength: tc.length,
			}
			if s.IsComplete() != tc.expected {
				t.Errorf("IsComplete() = %v, expected %v", s.IsComplete(), tc.expected)
			}
		})
	}
}

func TestUploadSession_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt string
		expected  bool
	}{
		{"future", time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339), false},
		{"past", time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339), true},
		{"now", time.Now().UTC().Format(time.RFC3339), true},
		{"invalid", "not-a-date", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &UploadSession{
				ExpiresAt: tc.expiresAt,
			}
			if s.IsExpired() != tc.expected {
				t.Errorf("IsExpired() = %v, expected %v", s.IsExpired(), tc.expected)
			}
		})
	}
}

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Create a temp dir for the test
	dir, err := os.MkdirTemp("", "tus-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open database: %v", err)
	}

	// Create schema
	schema := `
		CREATE TABLE IF NOT EXISTS storage_buckets (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			owner_id TEXT,
			public INTEGER DEFAULT 0,
			file_size_limit INTEGER,
			allowed_mime_types TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS _resumable_uploads (
			id TEXT PRIMARY KEY,
			bucket_id TEXT NOT NULL,
			object_name TEXT NOT NULL,
			owner_id TEXT,
			upload_length INTEGER NOT NULL,
			upload_offset INTEGER NOT NULL DEFAULT 0,
			content_type TEXT,
			cache_control TEXT,
			metadata TEXT DEFAULT '{}',
			upsert INTEGER DEFAULT 0,
			temp_path TEXT,
			s3_upload_id TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			expires_at TEXT NOT NULL,
			UNIQUE(bucket_id, object_name)
		);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.RemoveAll(dir)
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert test bucket
	_, err = db.Exec(`INSERT INTO storage_buckets (id, name) VALUES ('test-bucket-id', 'test-bucket')`)
	if err != nil {
		db.Close()
		os.RemoveAll(dir)
		t.Fatalf("failed to insert test bucket: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return db, cleanup
}

func TestTUSService_CreateUpload(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create temp dir for uploads
	uploadsDir, err := os.MkdirTemp("", "tus-uploads-*")
	if err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}
	defer os.RemoveAll(uploadsDir)

	cfg := DefaultTUSConfig()
	svc := NewTUSService(db, nil, cfg, uploadsDir)

	req := CreateUploadRequest{
		BucketID:     "test-bucket-id",
		ObjectName:   "test/file.txt",
		UploadLength: 1000,
		ContentType:  "text/plain",
		OwnerID:      "user-123",
		Upsert:       false,
	}

	session, err := svc.CreateUpload(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.BucketID != req.BucketID {
		t.Errorf("BucketID = %s, expected %s", session.BucketID, req.BucketID)
	}
	if session.ObjectName != req.ObjectName {
		t.Errorf("ObjectName = %s, expected %s", session.ObjectName, req.ObjectName)
	}
	if session.UploadLength != req.UploadLength {
		t.Errorf("UploadLength = %d, expected %d", session.UploadLength, req.UploadLength)
	}
	if session.UploadOffset != 0 {
		t.Errorf("UploadOffset = %d, expected 0", session.UploadOffset)
	}
	if session.TempPath == "" {
		t.Error("TempPath should not be empty")
	}

	// Verify temp file was created
	if _, err := os.Stat(session.TempPath); os.IsNotExist(err) {
		t.Error("temp file was not created")
	}
}

func TestTUSService_GetUpload(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	uploadsDir, err := os.MkdirTemp("", "tus-uploads-*")
	if err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}
	defer os.RemoveAll(uploadsDir)

	cfg := DefaultTUSConfig()
	svc := NewTUSService(db, nil, cfg, uploadsDir)

	// Create an upload
	req := CreateUploadRequest{
		BucketID:     "test-bucket-id",
		ObjectName:   "test/file.txt",
		UploadLength: 1000,
	}
	session, _ := svc.CreateUpload(context.Background(), req)

	// Retrieve it
	retrieved, err := svc.GetUpload(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetUpload failed: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("ID = %s, expected %s", retrieved.ID, session.ID)
	}
	if retrieved.UploadLength != session.UploadLength {
		t.Errorf("UploadLength = %d, expected %d", retrieved.UploadLength, session.UploadLength)
	}

	// Test not found
	_, err = svc.GetUpload(context.Background(), "nonexistent")
	if err != ErrTUSSessionNotFound {
		t.Errorf("expected ErrTUSSessionNotFound, got %v", err)
	}
}

func TestTUSService_WriteChunk(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	uploadsDir, err := os.MkdirTemp("", "tus-uploads-*")
	if err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}
	defer os.RemoveAll(uploadsDir)

	cfg := DefaultTUSConfig()
	svc := NewTUSService(db, nil, cfg, uploadsDir)

	// Create an upload
	req := CreateUploadRequest{
		BucketID:     "test-bucket-id",
		ObjectName:   "test/file.txt",
		UploadLength: 100,
	}
	session, _ := svc.CreateUpload(context.Background(), req)

	// Write first chunk
	chunk1 := strings.NewReader("Hello, ")
	newOffset, err := svc.WriteChunk(context.Background(), session.ID, 0, chunk1)
	if err != nil {
		t.Fatalf("WriteChunk failed: %v", err)
	}
	if newOffset != 7 {
		t.Errorf("newOffset = %d, expected 7", newOffset)
	}

	// Write second chunk
	chunk2 := strings.NewReader("World!")
	newOffset, err = svc.WriteChunk(context.Background(), session.ID, 7, chunk2)
	if err != nil {
		t.Fatalf("WriteChunk failed: %v", err)
	}
	if newOffset != 13 {
		t.Errorf("newOffset = %d, expected 13", newOffset)
	}

	// Verify file contents
	content, _ := os.ReadFile(session.TempPath)
	if string(content) != "Hello, World!" {
		t.Errorf("file content = %s, expected 'Hello, World!'", string(content))
	}

	// Test offset mismatch
	wrongChunk := strings.NewReader("test")
	_, err = svc.WriteChunk(context.Background(), session.ID, 0, wrongChunk)
	if err != ErrTUSOffsetMismatch {
		t.Errorf("expected ErrTUSOffsetMismatch, got %v", err)
	}
}

func TestTUSService_CancelUpload(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	uploadsDir, err := os.MkdirTemp("", "tus-uploads-*")
	if err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}
	defer os.RemoveAll(uploadsDir)

	cfg := DefaultTUSConfig()
	svc := NewTUSService(db, nil, cfg, uploadsDir)

	// Create an upload
	req := CreateUploadRequest{
		BucketID:     "test-bucket-id",
		ObjectName:   "test/file.txt",
		UploadLength: 100,
	}
	session, _ := svc.CreateUpload(context.Background(), req)
	tempPath := session.TempPath

	// Cancel it
	err = svc.CancelUpload(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("CancelUpload failed: %v", err)
	}

	// Verify session is gone
	_, err = svc.GetUpload(context.Background(), session.ID)
	if err != ErrTUSSessionNotFound {
		t.Errorf("expected ErrTUSSessionNotFound after cancel, got %v", err)
	}

	// Verify temp file is deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be deleted after cancel")
	}
}

func TestTUSService_CleanupExpired(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	uploadsDir, err := os.MkdirTemp("", "tus-uploads-*")
	if err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}
	defer os.RemoveAll(uploadsDir)

	cfg := TUSConfig{
		MaxSize:            0,
		ExpirationDuration: -1 * time.Hour, // Expired immediately
	}
	svc := NewTUSService(db, nil, cfg, uploadsDir)

	// Create an expired upload
	req := CreateUploadRequest{
		BucketID:     "test-bucket-id",
		ObjectName:   "test/expired.txt",
		UploadLength: 100,
	}
	session, _ := svc.CreateUpload(context.Background(), req)
	tempPath := session.TempPath

	// Run cleanup
	count, err := svc.CleanupExpired(context.Background())
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}
	if count != 1 {
		t.Errorf("cleaned up %d, expected 1", count)
	}

	// Verify temp file is deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be deleted after cleanup")
	}
}

func TestDefaultTUSConfig(t *testing.T) {
	cfg := DefaultTUSConfig()

	if cfg.MaxSize != 0 {
		t.Errorf("MaxSize = %d, expected 0 (unlimited)", cfg.MaxSize)
	}
	if cfg.ChunkSize != 5*1024*1024 {
		t.Errorf("ChunkSize = %d, expected %d", cfg.ChunkSize, 5*1024*1024)
	}
	if cfg.ExpirationDuration != 24*time.Hour {
		t.Errorf("ExpirationDuration = %v, expected 24h", cfg.ExpirationDuration)
	}
}

// Helper to read entire body
func readAll(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}
