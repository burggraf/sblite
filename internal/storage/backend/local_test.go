package backend

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalBackend(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "sblite-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewLocal(tmpDir)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("Write and Read", func(t *testing.T) {
		content := []byte("Hello, World!")
		key := "test/hello.txt"

		// Write file
		info, err := backend.Write(ctx, key, bytes.NewReader(content), int64(len(content)), "text/plain")
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if info.Key != key {
			t.Errorf("expected key %q, got %q", key, info.Key)
		}
		if info.Size != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), info.Size)
		}
		if info.ETag == "" {
			t.Error("expected non-empty ETag")
		}

		// Read file
		reader, readInfo, err := backend.Reader(ctx, key)
		if err != nil {
			t.Fatalf("Reader failed: %v", err)
		}
		defer reader.Close()

		readContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(readContent, content) {
			t.Errorf("content mismatch: expected %q, got %q", content, readContent)
		}
		if readInfo.Size != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), readInfo.Size)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		key := "test/exists.txt"
		content := []byte("test content")

		// File doesn't exist yet
		exists, err := backend.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected file to not exist")
		}

		// Write file
		_, err = backend.Write(ctx, key, bytes.NewReader(content), int64(len(content)), "text/plain")
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// File should exist now
		exists, err = backend.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected file to exist")
		}
	})

	t.Run("Attributes", func(t *testing.T) {
		key := "test/attrs.txt"
		content := []byte("attribute test")

		_, err := backend.Write(ctx, key, bytes.NewReader(content), int64(len(content)), "text/plain")
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		attrs, err := backend.Attributes(ctx, key)
		if err != nil {
			t.Fatalf("Attributes failed: %v", err)
		}

		if attrs.Key != key {
			t.Errorf("expected key %q, got %q", key, attrs.Key)
		}
		if attrs.Size != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), attrs.Size)
		}
		if attrs.ETag == "" {
			t.Error("expected non-empty ETag")
		}
	})

	t.Run("Attributes NotFound", func(t *testing.T) {
		_, err := backend.Attributes(ctx, "nonexistent/file.txt")
		if !IsNotFound(err) {
			t.Errorf("expected NotFound error, got: %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key := "test/delete.txt"
		content := []byte("delete me")

		_, err := backend.Write(ctx, key, bytes.NewReader(content), int64(len(content)), "text/plain")
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Verify file exists
		exists, _ := backend.Exists(ctx, key)
		if !exists {
			t.Fatal("file should exist before delete")
		}

		// Delete file
		err = backend.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file is gone
		exists, _ = backend.Exists(ctx, key)
		if exists {
			t.Error("file should not exist after delete")
		}

		// Delete again should not error (idempotent)
		err = backend.Delete(ctx, key)
		if err != nil {
			t.Errorf("second Delete should not error: %v", err)
		}
	})

	t.Run("Copy", func(t *testing.T) {
		srcKey := "test/copy-src.txt"
		dstKey := "test/copy-dst.txt"
		content := []byte("copy this content")

		_, err := backend.Write(ctx, srcKey, bytes.NewReader(content), int64(len(content)), "text/plain")
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Copy file
		err = backend.Copy(ctx, srcKey, dstKey)
		if err != nil {
			t.Fatalf("Copy failed: %v", err)
		}

		// Verify destination exists
		reader, _, err := backend.Reader(ctx, dstKey)
		if err != nil {
			t.Fatalf("Reader failed: %v", err)
		}
		defer reader.Close()

		readContent, _ := io.ReadAll(reader)
		if !bytes.Equal(readContent, content) {
			t.Errorf("copied content mismatch: expected %q, got %q", content, readContent)
		}

		// Source should still exist
		exists, _ := backend.Exists(ctx, srcKey)
		if !exists {
			t.Error("source file should still exist after copy")
		}
	})

	t.Run("List", func(t *testing.T) {
		// Create several files
		files := []string{"list/a.txt", "list/b.txt", "list/c.txt", "list/sub/d.txt"}
		for _, key := range files {
			_, err := backend.Write(ctx, key, bytes.NewReader([]byte("content")), 7, "text/plain")
			if err != nil {
				t.Fatalf("Write failed for %s: %v", key, err)
			}
		}

		// List with prefix
		results, cursor, err := backend.List(ctx, "list/", 10, "")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(results) != 4 {
			t.Errorf("expected 4 results, got %d", len(results))
		}
		if cursor != "" {
			t.Errorf("expected empty cursor, got %q", cursor)
		}

		// Verify all files are in results
		resultKeys := make(map[string]bool)
		for _, r := range results {
			resultKeys[r.Key] = true
		}
		for _, key := range files {
			if !resultKeys[key] {
				t.Errorf("missing file in results: %s", key)
			}
		}
	})

	t.Run("List with pagination", func(t *testing.T) {
		// Create files
		for i := 0; i < 5; i++ {
			key := filepath.Join("paginate", string(rune('a'+i))+".txt")
			_, err := backend.Write(ctx, key, bytes.NewReader([]byte("x")), 1, "text/plain")
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}

		// List with limit
		results, cursor, err := backend.List(ctx, "paginate/", 2, "")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
		if cursor == "" {
			t.Error("expected non-empty cursor for pagination")
		}

		// Get next page
		results2, _, err := backend.List(ctx, "paginate/", 10, cursor)
		if err != nil {
			t.Fatalf("List page 2 failed: %v", err)
		}

		if len(results2) != 3 {
			t.Errorf("expected 3 results on page 2, got %d", len(results2))
		}
	})

	t.Run("DeletePrefix", func(t *testing.T) {
		// Create files
		prefix := "deleteprefix/"
		files := []string{prefix + "a.txt", prefix + "b.txt", prefix + "sub/c.txt"}
		for _, key := range files {
			_, err := backend.Write(ctx, key, bytes.NewReader([]byte("x")), 1, "text/plain")
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}

		// Delete all with prefix
		errors := backend.DeletePrefix(ctx, prefix)
		if len(errors) > 0 {
			t.Errorf("DeletePrefix had errors: %v", errors)
		}

		// Verify all are gone
		for _, key := range files {
			exists, _ := backend.Exists(ctx, key)
			if exists {
				t.Errorf("file should be deleted: %s", key)
			}
		}
	})
}

func TestLocalBackendPathTraversal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sblite-storage-security-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewLocal(tmpDir)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Test path traversal attacks
	maliciousKeys := []string{
		"../etc/passwd",
		"test/../../../etc/passwd",
		"/etc/passwd",
		"test/\x00/file.txt",
		"..\\windows\\system32",
	}

	for _, key := range maliciousKeys {
		t.Run("Key: "+strings.ReplaceAll(key, "\x00", "\\x00"), func(t *testing.T) {
			_, err := backend.Write(ctx, key, bytes.NewReader([]byte("x")), 1, "text/plain")
			if !IsInvalidKey(err) {
				t.Errorf("expected InvalidKey error for %q, got: %v", key, err)
			}

			_, err = backend.Exists(ctx, key)
			if !IsInvalidKey(err) {
				t.Errorf("Exists: expected InvalidKey error for %q, got: %v", key, err)
			}

			_, err = backend.Attributes(ctx, key)
			if !IsInvalidKey(err) {
				t.Errorf("Attributes: expected InvalidKey error for %q, got: %v", key, err)
			}
		})
	}
}

func TestLocalBackendEmptyKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sblite-storage-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewLocal(tmpDir)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	_, err = backend.Write(ctx, "", bytes.NewReader([]byte("x")), 1, "text/plain")
	if !IsInvalidKey(err) {
		t.Errorf("expected InvalidKey error for empty key, got: %v", err)
	}
}

func TestLocalBackendAtomicWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sblite-storage-atomic-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewLocal(tmpDir)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	key := "atomic/test.txt"

	// Write initial content
	_, err = backend.Write(ctx, key, bytes.NewReader([]byte("initial")), 7, "text/plain")
	if err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Overwrite with new content
	_, err = backend.Write(ctx, key, bytes.NewReader([]byte("updated content")), 15, "text/plain")
	if err != nil {
		t.Fatalf("update write failed: %v", err)
	}

	// Verify new content
	reader, _, err := backend.Reader(ctx, key)
	if err != nil {
		t.Fatalf("Reader failed: %v", err)
	}
	defer reader.Close()

	content, _ := io.ReadAll(reader)
	if string(content) != "updated content" {
		t.Errorf("expected 'updated content', got %q", string(content))
	}
}
