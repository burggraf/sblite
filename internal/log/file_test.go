// internal/log/file_test.go
package log

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileHandler_Write(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cfg := &Config{
		FilePath:   logPath,
		Format:     "text",
		MaxSizeMB:  1,
		MaxAgeDays: 7,
		MaxBackups: 3,
	}

	h, err := NewFileHandler(cfg, slog.LevelInfo)
	if err != nil {
		t.Fatalf("NewFileHandler: %v", err)
	}
	defer h.Close()

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	// Read the file
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "test message") {
		t.Errorf("expected file to contain 'test message', got %q", output)
	}
}

func TestFileHandler_Rotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cfg := &Config{
		FilePath:   logPath,
		Format:     "text",
		MaxSizeMB:  0, // Force immediate rotation (will use 1KB minimum)
		MaxAgeDays: 7,
		MaxBackups: 2,
	}

	h, err := NewFileHandler(cfg, slog.LevelInfo)
	if err != nil {
		t.Fatalf("NewFileHandler: %v", err)
	}
	defer h.Close()

	logger := slog.New(h)

	// Write enough to trigger rotation
	for i := 0; i < 100; i++ {
		logger.Info("test message with some padding to make it larger", "iteration", i)
	}

	// Force a rotation check
	h.checkRotate()

	// Check that backup files were created
	files, _ := filepath.Glob(filepath.Join(dir, "test.log*"))
	if len(files) < 2 {
		t.Errorf("expected at least 2 files after rotation, got %d", len(files))
	}
}
