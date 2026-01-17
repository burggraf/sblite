// internal/log/database_test.go
package log

import (
	"database/sql"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestDBHandler_Write(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-log.db")

	cfg := &Config{
		DBPath:        dbPath,
		RetentionDays: 7,
		Fields:        []string{"source", "request_id"},
	}

	h, err := NewDBHandler(cfg, slog.LevelInfo)
	if err != nil {
		t.Fatalf("NewDBHandler: %v", err)
	}
	defer h.Close()

	logger := slog.New(h)
	logger.Info("test message", "key", "value", "request_id", "abc123")

	// Query the database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 log entry, got %d", count)
	}

	var msg, level, reqID string
	err = db.QueryRow("SELECT message, level, request_id FROM logs").Scan(&msg, &level, &reqID)
	if err != nil {
		t.Fatalf("Query row: %v", err)
	}
	if msg != "test message" {
		t.Errorf("expected message 'test message', got %q", msg)
	}
	if level != "INFO" {
		t.Errorf("expected level 'INFO', got %q", level)
	}
	if reqID != "abc123" {
		t.Errorf("expected request_id 'abc123', got %q", reqID)
	}
}

func TestDBHandler_Retention(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-log.db")

	cfg := &Config{
		DBPath:        dbPath,
		RetentionDays: 0, // immediate cleanup
		Fields:        []string{},
	}

	h, err := NewDBHandler(cfg, slog.LevelInfo)
	if err != nil {
		t.Fatalf("NewDBHandler: %v", err)
	}
	defer h.Close()

	// Insert an old record directly
	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()

	oldTime := time.Now().AddDate(0, 0, -1).Format(time.RFC3339)
	db.Exec("INSERT INTO logs (timestamp, level, message) VALUES (?, 'INFO', 'old message')", oldTime)

	// Run cleanup
	h.runCleanup()

	// Check that old record is deleted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 logs after cleanup, got %d", count)
	}
}
