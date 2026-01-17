# Logging System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add configurable logging to sblite with console, file, and database backends using Go's slog.

**Architecture:** Create `internal/log` package with slog-based handlers. Each handler implements `slog.Handler` interface. Single active output at a time. File handler rotates by size with age-based cleanup. Database handler uses separate log.db with hourly retention cleanup.

**Tech Stack:** Go log/slog, modernc.org/sqlite, Chi middleware

---

## Task 1: Create Log Package Foundation

**Files:**
- Create: `internal/log/logger.go`
- Create: `internal/log/logger_test.go`

**Step 1: Write test for config defaults and level parsing**

```go
// internal/log/logger_test.go
package log

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mode != "console" {
		t.Errorf("expected mode 'console', got %q", cfg.Mode)
	}
	if cfg.Level != "info" {
		t.Errorf("expected level 'info', got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Errorf("expected format 'text', got %q", cfg.Format)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  int // slog.Level value
	}{
		{"debug", -4},
		{"info", 0},
		{"warn", 4},
		{"error", 8},
		{"invalid", 0}, // defaults to info
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if int(got) != tt.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log/... -v`
Expected: FAIL - package does not exist

**Step 3: Create logger.go with Config and ParseLevel**

```go
// internal/log/logger.go
package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Config holds all logging configuration.
type Config struct {
	Mode   string // "console", "file", "database"
	Level  string // "debug", "info", "warn", "error"
	Format string // "text", "json" (for console/file only)

	// File-specific
	FilePath   string
	MaxSizeMB  int // Rotate when file exceeds this size
	MaxAgeDays int // Delete files older than this
	MaxBackups int // Keep at most this many old files

	// Database-specific
	DBPath        string   // Path to log.db
	RetentionDays int      // Delete entries older than this
	Fields        []string // Optional fields: "source", "request_id", "user_id", "extra"
}

// DefaultConfig returns the default logging configuration.
func DefaultConfig() *Config {
	return &Config{
		Mode:          "console",
		Level:         "info",
		Format:        "text",
		FilePath:      "sblite.log",
		MaxSizeMB:     100,
		MaxAgeDays:    7,
		MaxBackups:    3,
		DBPath:        "log.db",
		RetentionDays: 7,
		Fields:        []string{},
	}
}

// ParseLevel converts a string level to slog.Level.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

var (
	defaultLogger *slog.Logger
	mu            sync.RWMutex
)

// Init initializes the global logger with the given configuration.
func Init(cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()

	var handler slog.Handler
	level := ParseLevel(cfg.Level)

	switch cfg.Mode {
	case "file":
		h, err := NewFileHandler(cfg, level)
		if err != nil {
			return err
		}
		handler = h
	case "database":
		h, err := NewDBHandler(cfg, level)
		if err != nil {
			return err
		}
		handler = h
	default:
		handler = NewConsoleHandler(os.Stdout, cfg, level)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	return nil
}

// Logger returns the current default logger.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if defaultLogger == nil {
		return slog.Default()
	}
	return defaultLogger
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

// With returns a logger with the given attributes.
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

// Log logs at the given level.
func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	Logger().Log(ctx, level, msg, args...)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/log/... -v`
Expected: FAIL - NewFileHandler, NewDBHandler, NewConsoleHandler not defined (that's ok for now)

**Step 5: Create stub handlers to make tests pass**

Add to `internal/log/logger.go` (temporary stubs):

```go
// Stub handlers - will be replaced in subsequent tasks
func NewConsoleHandler(w io.Writer, cfg *Config, level slog.Level) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
}

func NewFileHandler(cfg *Config, level slog.Level) (slog.Handler, error) {
	return nil, fmt.Errorf("file handler not implemented")
}

func NewDBHandler(cfg *Config, level slog.Level) (slog.Handler, error) {
	return nil, fmt.Errorf("database handler not implemented")
}
```

Add imports: `"fmt"`, `"io"`

**Step 6: Run test to verify it passes**

Run: `go test ./internal/log/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/log/
git commit -m "feat(log): add logging package foundation with config and level parsing"
```

---

## Task 2: Console Handler

**Files:**
- Create: `internal/log/console.go`
- Create: `internal/log/console_test.go`
- Modify: `internal/log/logger.go` - remove stub

**Step 1: Write test for console handler**

```go
// internal/log/console_test.go
package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestConsoleHandler_Text(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	h := NewConsoleHandler(&buf, cfg, slog.LevelInfo)

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %q", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected output to contain 'key=value', got %q", output)
	}
}

func TestConsoleHandler_JSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "json"}
	h := NewConsoleHandler(&buf, cfg, slog.LevelInfo)

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("expected JSON output with msg field, got %q", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("expected JSON output with key field, got %q", output)
	}
}

func TestConsoleHandler_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	h := NewConsoleHandler(&buf, cfg, slog.LevelWarn)

	logger := slog.New(h)
	logger.Info("should not appear")
	logger.Warn("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Errorf("info message should be filtered out at warn level")
	}
	if !strings.Contains(output, "should appear") {
		t.Errorf("warn message should appear")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log/... -v -run Console`
Expected: FAIL - console handler uses stub

**Step 3: Create console.go**

```go
// internal/log/console.go
package log

import (
	"io"
	"log/slog"
)

// NewConsoleHandler creates a handler that writes to the given writer.
// Format can be "text" or "json".
func NewConsoleHandler(w io.Writer, cfg *Config, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if cfg.Format == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}
```

**Step 4: Remove stub from logger.go**

Remove the `NewConsoleHandler` stub function from `logger.go`.

**Step 5: Run test to verify it passes**

Run: `go test ./internal/log/... -v -run Console`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/log/console.go internal/log/console_test.go internal/log/logger.go
git commit -m "feat(log): add console handler with text and JSON formats"
```

---

## Task 3: File Handler with Rotation

**Files:**
- Create: `internal/log/file.go`
- Create: `internal/log/file_test.go`
- Modify: `internal/log/logger.go` - remove stub

**Step 1: Write test for file handler**

```go
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
	h.(*FileHandler).checkRotate()

	// Check that backup files were created
	files, _ := filepath.Glob(filepath.Join(dir, "test.log*"))
	if len(files) < 2 {
		t.Errorf("expected at least 2 files after rotation, got %d", len(files))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log/... -v -run File`
Expected: FAIL - file handler not implemented

**Step 3: Create file.go**

```go
// internal/log/file.go
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileHandler writes logs to a file with rotation support.
type FileHandler struct {
	mu         sync.Mutex
	file       *os.File
	path       string
	maxSize    int64 // bytes
	maxAge     int   // days
	maxBackups int
	size       int64
	format     string
	level      slog.Level
	inner      slog.Handler
}

// NewFileHandler creates a file handler with rotation.
func NewFileHandler(cfg *Config, level slog.Level) (*FileHandler, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.FilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
	}

	file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat log file: %w", err)
	}

	maxSize := int64(cfg.MaxSizeMB) * 1024 * 1024
	if maxSize < 1024 {
		maxSize = 1024 // minimum 1KB for testing
	}

	h := &FileHandler{
		file:       file,
		path:       cfg.FilePath,
		maxSize:    maxSize,
		maxAge:     cfg.MaxAgeDays,
		maxBackups: cfg.MaxBackups,
		size:       info.Size(),
		format:     cfg.Format,
		level:      level,
	}

	// Create inner handler for formatting
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "json" {
		h.inner = slog.NewJSONHandler(file, opts)
	} else {
		h.inner = slog.NewTextHandler(file, opts)
	}

	return h, nil
}

// Enabled reports whether the handler handles records at the given level.
func (h *FileHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle writes the record to the file, rotating if necessary.
func (h *FileHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.size >= h.maxSize {
		if err := h.rotate(); err != nil {
			return err
		}
	}

	// Get current file position before write
	pos, _ := h.file.Seek(0, io.SeekCurrent)

	err := h.inner.Handle(ctx, r)

	// Update size based on new position
	newPos, _ := h.file.Seek(0, io.SeekCurrent)
	h.size += newPos - pos

	return err
}

// WithAttrs returns a new handler with the given attributes.
func (h *FileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()

	return &FileHandler{
		file:       h.file,
		path:       h.path,
		maxSize:    h.maxSize,
		maxAge:     h.maxAge,
		maxBackups: h.maxBackups,
		size:       h.size,
		format:     h.format,
		level:      h.level,
		inner:      h.inner.WithAttrs(attrs),
	}
}

// WithGroup returns a new handler with the given group.
func (h *FileHandler) WithGroup(name string) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()

	return &FileHandler{
		file:       h.file,
		path:       h.path,
		maxSize:    h.maxSize,
		maxAge:     h.maxAge,
		maxBackups: h.maxBackups,
		size:       h.size,
		format:     h.format,
		level:      h.level,
		inner:      h.inner.WithGroup(name),
	}
}

// rotate closes the current file and creates a new one.
func (h *FileHandler) rotate() error {
	h.file.Close()

	// Rename current file with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupPath := h.path + "." + timestamp
	if err := os.Rename(h.path, backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rename log file: %w", err)
	}

	// Clean old backups
	h.cleanOldBackups()

	// Create new file
	file, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("create new log file: %w", err)
	}

	h.file = file
	h.size = 0

	// Recreate inner handler with new file
	opts := &slog.HandlerOptions{Level: h.level}
	if h.format == "json" {
		h.inner = slog.NewJSONHandler(file, opts)
	} else {
		h.inner = slog.NewTextHandler(file, opts)
	}

	return nil
}

// cleanOldBackups removes backup files exceeding maxBackups or older than maxAge.
func (h *FileHandler) cleanOldBackups() {
	pattern := h.path + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	// Sort by modification time, newest first
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})

	cutoff := time.Now().AddDate(0, 0, -h.maxAge)

	for i, path := range matches {
		// Keep only maxBackups files
		if i >= h.maxBackups {
			os.Remove(path)
			continue
		}

		// Remove files older than maxAge
		info, err := os.Stat(path)
		if err == nil && info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
	}
}

// checkRotate checks if rotation is needed and performs it.
// Exported for testing.
func (h *FileHandler) checkRotate() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.size >= h.maxSize {
		h.rotate()
	}
}

// Close closes the file handler.
func (h *FileHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.file != nil {
		return h.file.Close()
	}
	return nil
}

// Closeable interface for handlers that need cleanup.
type Closeable interface {
	Close() error
}
```

**Step 4: Update logger.go to remove file handler stub**

Remove the `NewFileHandler` stub and update Init to handle the return type:

```go
// In Init function, update file case:
case "file":
	h, err := NewFileHandler(cfg, level)
	if err != nil {
		return err
	}
	handler = h
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/log/... -v -run File`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/log/file.go internal/log/file_test.go internal/log/logger.go
git commit -m "feat(log): add file handler with size-based rotation"
```

---

## Task 4: Database Handler

**Files:**
- Create: `internal/log/database.go`
- Create: `internal/log/database_test.go`
- Modify: `internal/log/logger.go` - remove stub

**Step 1: Write test for database handler**

```go
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
	h.(*DBHandler).runCleanup()

	// Check that old record is deleted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 logs after cleanup, got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log/... -v -run DB`
Expected: FAIL - database handler not implemented

**Step 3: Create database.go**

```go
// internal/log/database.go
package log

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const createLogsTableSQL = `
CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    source TEXT,
    request_id TEXT,
    user_id TEXT,
    extra TEXT
);
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
`

// DBHandler writes logs to a SQLite database.
type DBHandler struct {
	mu            sync.Mutex
	db            *sql.DB
	stmt          *sql.Stmt
	retention     int
	fields        map[string]bool
	level         slog.Level
	cleanupTicker *time.Ticker
	done          chan struct{}
}

// NewDBHandler creates a database handler.
func NewDBHandler(cfg *Config, level slog.Level) (*DBHandler, error) {
	db, err := sql.Open("sqlite", cfg.DBPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open log database: %w", err)
	}

	// Create table
	if _, err := db.Exec(createLogsTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create logs table: %w", err)
	}

	// Prepare insert statement
	stmt, err := db.Prepare(`
		INSERT INTO logs (timestamp, level, message, source, request_id, user_id, extra)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare insert: %w", err)
	}

	fields := make(map[string]bool)
	for _, f := range cfg.Fields {
		fields[f] = true
	}

	h := &DBHandler{
		db:        db,
		stmt:      stmt,
		retention: cfg.RetentionDays,
		fields:    fields,
		level:     level,
		done:      make(chan struct{}),
	}

	// Start cleanup ticker
	h.startCleanup()

	return h, nil
}

// Enabled reports whether the handler handles records at the given level.
func (h *DBHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle writes the record to the database.
func (h *DBHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Extract fields from attributes
	var source, requestID, userID, extra sql.NullString

	if h.fields["source"] {
		// Get caller info
		_, file, line, ok := runtime.Caller(4) // Skip through slog internals
		if ok {
			source = sql.NullString{String: fmt.Sprintf("%s:%d", file, line), Valid: true}
		}
	}

	extraData := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "request_id":
			if h.fields["request_id"] {
				requestID = sql.NullString{String: a.Value.String(), Valid: true}
			}
		case "user_id":
			if h.fields["user_id"] {
				userID = sql.NullString{String: a.Value.String(), Valid: true}
			}
		default:
			if h.fields["extra"] {
				extraData[a.Key] = a.Value.Any()
			}
		}
		return true
	})

	if h.fields["extra"] && len(extraData) > 0 {
		data, _ := json.Marshal(extraData)
		extra = sql.NullString{String: string(data), Valid: true}
	}

	_, err := h.stmt.Exec(
		r.Time.Format(time.RFC3339),
		r.Level.String(),
		r.Message,
		source,
		requestID,
		userID,
		extra,
	)
	return err
}

// WithAttrs returns a new handler with the given attributes.
func (h *DBHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, return same handler - attrs handled in Handle
	return h
}

// WithGroup returns a new handler with the given group.
func (h *DBHandler) WithGroup(name string) slog.Handler {
	// For simplicity, return same handler
	return h
}

// startCleanup starts the background cleanup ticker.
func (h *DBHandler) startCleanup() {
	h.cleanupTicker = time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-h.cleanupTicker.C:
				h.runCleanup()
			case <-h.done:
				return
			}
		}
	}()
}

// runCleanup deletes old log entries.
func (h *DBHandler) runCleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -h.retention)
	h.db.Exec("DELETE FROM logs WHERE timestamp < ?", cutoff.Format(time.RFC3339))
}

// Close closes the database handler.
func (h *DBHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	close(h.done)
	if h.cleanupTicker != nil {
		h.cleanupTicker.Stop()
	}
	if h.stmt != nil {
		h.stmt.Close()
	}
	if h.db != nil {
		return h.db.Close()
	}
	return nil
}
```

**Step 4: Update logger.go to remove database handler stub**

Remove the `NewDBHandler` stub function.

**Step 5: Run test to verify it passes**

Run: `go test ./internal/log/... -v -run DB`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/log/database.go internal/log/database_test.go internal/log/logger.go
git commit -m "feat(log): add database handler with SQLite storage and retention"
```

---

## Task 5: HTTP Request Logging Middleware

**Files:**
- Create: `internal/log/middleware.go`
- Create: `internal/log/middleware_test.go`

**Step 1: Write test for request logging middleware**

```go
// internal/log/middleware_test.go
package log

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	handler := NewConsoleHandler(&buf, cfg, slog.LevelInfo)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Wrap with our middleware
	wrapped := RequestLogger(testHandler)

	// Make a request
	req := httptest.NewRequest("GET", "/test/path", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check log output
	output := buf.String()
	if !strings.Contains(output, "http request") {
		t.Errorf("expected log to contain 'http request', got %q", output)
	}
	if !strings.Contains(output, "GET") {
		t.Errorf("expected log to contain 'GET', got %q", output)
	}
	if !strings.Contains(output, "/test/path") {
		t.Errorf("expected log to contain '/test/path', got %q", output)
	}
	if !strings.Contains(output, "status=200") {
		t.Errorf("expected log to contain 'status=200', got %q", output)
	}
}

func TestRequestLogger_ErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	handler := NewConsoleHandler(&buf, cfg, slog.LevelInfo)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	wrapped := RequestLogger(testHandler)

	req := httptest.NewRequest("GET", "/error", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	output := buf.String()
	if !strings.Contains(output, "level=ERROR") {
		t.Errorf("expected ERROR level for 500 status, got %q", output)
	}
}

func TestGetRequestID(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := GetRequestID(r.Context())
		if reqID == "" {
			t.Error("expected request ID in context")
		}
		if len(reqID) != 8 {
			t.Errorf("expected 8-char request ID, got %d chars", len(reqID))
		}
	})

	wrapped := RequestLogger(testHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log/... -v -run Request`
Expected: FAIL - RequestLogger not defined

**Step 3: Create middleware.go**

```go
// internal/log/middleware.go
package log

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request_id"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// RequestLogger returns middleware that logs HTTP requests.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate request ID
		reqID := uuid.NewString()[:8]

		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)

		// Wrap response writer to capture status
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Call next handler
		next.ServeHTTP(ww, r.WithContext(ctx))

		// Calculate duration
		duration := time.Since(start)

		// Determine log level based on status
		level := slog.LevelInfo
		if ww.status >= 500 {
			level = slog.LevelError
		} else if ww.status >= 400 {
			level = slog.LevelWarn
		}

		// Log the request
		slog.Log(ctx, level, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"duration_ms", duration.Milliseconds(),
			"request_id", reqID,
			"remote_addr", r.RemoteAddr,
		)
	})
}

// GetRequestID returns the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/log/... -v -run Request`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/log/middleware.go internal/log/middleware_test.go
git commit -m "feat(log): add HTTP request logging middleware"
```

---

## Task 6: CLI Flags and Environment Variables

**Files:**
- Modify: `cmd/serve.go`

**Step 1: Read current serve.go**

Already read earlier. We need to add log flags and config building.

**Step 2: Add log flags to init()**

Add after existing flags in `init()`:

```go
// Logging flags
serveCmd.Flags().String("log-mode", "", "Logging output: console, file, database (default: console)")
serveCmd.Flags().String("log-level", "", "Log level: debug, info, warn, error (default: info)")
serveCmd.Flags().String("log-format", "", "Log format: text, json (default: text)")
serveCmd.Flags().String("log-file", "", "Log file path (default: sblite.log)")
serveCmd.Flags().String("log-db", "", "Log database path (default: log.db)")
serveCmd.Flags().Int("log-max-size", 0, "Max log file size in MB (default: 100)")
serveCmd.Flags().Int("log-max-age", 0, "Max age of logs in days (default: 7)")
serveCmd.Flags().Int("log-max-backups", 0, "Max backup files to keep (default: 3)")
serveCmd.Flags().StringSlice("log-fields", nil, "DB log fields: source,request_id,user_id,extra")
```

**Step 3: Add buildLogConfig function**

```go
// buildLogConfig creates a log.Config from environment variables and CLI flags.
func buildLogConfig(cmd *cobra.Command) *log.Config {
	cfg := log.DefaultConfig()

	// Read environment variables first
	if mode := os.Getenv("SBLITE_LOG_MODE"); mode != "" {
		cfg.Mode = mode
	}
	if level := os.Getenv("SBLITE_LOG_LEVEL"); level != "" {
		cfg.Level = level
	}
	if format := os.Getenv("SBLITE_LOG_FORMAT"); format != "" {
		cfg.Format = format
	}
	if filePath := os.Getenv("SBLITE_LOG_FILE"); filePath != "" {
		cfg.FilePath = filePath
	}
	if dbPath := os.Getenv("SBLITE_LOG_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}
	if maxSize := os.Getenv("SBLITE_LOG_MAX_SIZE"); maxSize != "" {
		if v, err := strconv.Atoi(maxSize); err == nil {
			cfg.MaxSizeMB = v
		}
	}
	if maxAge := os.Getenv("SBLITE_LOG_MAX_AGE"); maxAge != "" {
		if v, err := strconv.Atoi(maxAge); err == nil {
			cfg.MaxAgeDays = v
		}
	}
	if maxBackups := os.Getenv("SBLITE_LOG_MAX_BACKUPS"); maxBackups != "" {
		if v, err := strconv.Atoi(maxBackups); err == nil {
			cfg.MaxBackups = v
		}
	}
	if fields := os.Getenv("SBLITE_LOG_FIELDS"); fields != "" {
		cfg.Fields = strings.Split(fields, ",")
	}

	// CLI flags override environment variables
	if mode, _ := cmd.Flags().GetString("log-mode"); mode != "" {
		cfg.Mode = mode
	}
	if level, _ := cmd.Flags().GetString("log-level"); level != "" {
		cfg.Level = level
	}
	if format, _ := cmd.Flags().GetString("log-format"); format != "" {
		cfg.Format = format
	}
	if filePath, _ := cmd.Flags().GetString("log-file"); filePath != "" {
		cfg.FilePath = filePath
	}
	if dbPath, _ := cmd.Flags().GetString("log-db"); dbPath != "" {
		cfg.DBPath = dbPath
	}
	if maxSize, _ := cmd.Flags().GetInt("log-max-size"); maxSize > 0 {
		cfg.MaxSizeMB = maxSize
	}
	if maxAge, _ := cmd.Flags().GetInt("log-max-age"); maxAge > 0 {
		cfg.MaxAgeDays = maxAge
	}
	if maxBackups, _ := cmd.Flags().GetInt("log-max-backups"); maxBackups > 0 {
		cfg.MaxBackups = maxBackups
	}
	if fields, _ := cmd.Flags().GetStringSlice("log-fields"); len(fields) > 0 {
		cfg.Fields = fields
	}

	return cfg
}
```

**Step 4: Update RunE to initialize logging**

Add at the beginning of RunE, before other code:

```go
// Initialize logging
logConfig := buildLogConfig(cmd)
if err := log.Init(logConfig); err != nil {
	return fmt.Errorf("failed to initialize logging: %w", err)
}
```

**Step 5: Add import**

```go
import (
	// ... existing imports ...
	"strings"

	"github.com/markb/sblite/internal/log"
)
```

**Step 6: Build and verify**

Run: `go build -o sblite . && ./sblite serve --help`
Expected: See new logging flags in help output

**Step 7: Commit**

```bash
git add cmd/serve.go
git commit -m "feat(cli): add logging configuration flags and env vars"
```

---

## Task 7: Replace fmt.Printf with log calls

**Files:**
- Modify: `cmd/serve.go`
- Modify: `internal/server/auth_handlers.go`

**Step 1: Update cmd/serve.go**

Replace fmt.Printf/Println calls with log calls:

```go
// Before:
fmt.Println("Warning: Using default JWT secret. Set SBLITE_JWT_SECRET in production.")

// After:
log.Warn("using default JWT secret, set SBLITE_JWT_SECRET in production")

// Before:
fmt.Printf("Starting Supabase Lite on %s\n", addr)
fmt.Printf("  Auth API: http://%s/auth/v1\n", addr)
fmt.Printf("  REST API: http://%s/rest/v1\n", addr)
fmt.Printf("  Mail Mode: %s\n", mailConfig.Mode)

// After:
log.Info("starting server",
	"addr", addr,
	"auth_api", "http://"+addr+"/auth/v1",
	"rest_api", "http://"+addr+"/rest/v1",
	"mail_mode", mailConfig.Mode,
)

// Before (mail UI line):
fmt.Printf("  Mail UI: http://%s/mail\n", addr)

// After:
log.Info("mail UI available", "url", "http://"+addr+"/mail")
```

**Step 2: Update internal/server/auth_handlers.go**

```go
// Before (line ~460):
fmt.Printf("Failed to send magic link email: %v\n", err)

// After:
// (Need to import log package first)
// log.Error("failed to send magic link email", "error", err)

// Before (line ~507):
fmt.Printf("Failed to send invite email: %v\n", err)

// After:
// log.Error("failed to send invite email", "error", err)
```

Note: These are inside the server package, not importing our log package. We have two options:
1. Import our log package
2. Use slog directly (since we set it as default)

Use slog directly to avoid circular dependency risks:

```go
import "log/slog"

// Replace:
fmt.Printf("Failed to send magic link email: %v\n", err)
// With:
slog.Error("failed to send magic link email", "error", err)
```

**Step 3: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Expected: See structured log output instead of fmt.Printf output

**Step 4: Commit**

```bash
git add cmd/serve.go internal/server/auth_handlers.go
git commit -m "refactor: replace fmt.Printf with structured logging"
```

---

## Task 8: Replace Chi middleware.Logger

**Files:**
- Modify: `internal/server/server.go`

**Step 1: Update imports**

```go
import (
	// ... existing imports ...
	"github.com/markb/sblite/internal/log"
)
```

**Step 2: Replace middleware.Logger**

```go
// Before:
s.router.Use(middleware.Logger)

// After:
s.router.Use(log.RequestLogger)
```

**Step 3: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Make a request: `curl http://localhost:8080/health`
Expected: See structured request log with method, path, status, duration

**Step 4: Commit**

```bash
git add internal/server/server.go
git commit -m "refactor: use custom request logger instead of chi middleware.Logger"
```

---

## Task 9: Update CLAUDE.md Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add logging section**

Add after "Environment Variables" section:

```markdown
### Logging Configuration

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--log-mode` | `SBLITE_LOG_MODE` | `console` | Output: console, file, database |
| `--log-level` | `SBLITE_LOG_LEVEL` | `info` | Level: debug, info, warn, error |
| `--log-format` | `SBLITE_LOG_FORMAT` | `text` | Format: text, json |
| `--log-file` | `SBLITE_LOG_FILE` | `sblite.log` | Log file path |
| `--log-db` | `SBLITE_LOG_DB` | `log.db` | Log database path |
| `--log-max-size` | `SBLITE_LOG_MAX_SIZE` | `100` | Max file size (MB) |
| `--log-max-age` | `SBLITE_LOG_MAX_AGE` | `7` | Retention days |
| `--log-max-backups` | `SBLITE_LOG_MAX_BACKUPS` | `3` | Backup files to keep |
| `--log-fields` | `SBLITE_LOG_FIELDS` | `` | DB fields (comma-separated) |

**Example usage:**
```bash
# JSON console logging
./sblite serve --log-format=json

# File logging with rotation
./sblite serve --log-mode=file --log-file=/var/log/sblite.log

# Database logging with full context
./sblite serve --log-mode=database --log-db=/var/log/sblite-log.db \
  --log-fields=source,request_id,user_id,extra
```
```

**Step 2: Update project structure**

Add to the structure:

```
│   ├── log/                  # Logging system
│   │   ├── logger.go         # Config, initialization
│   │   ├── console.go        # Console handler
│   │   ├── file.go           # File handler with rotation
│   │   ├── database.go       # SQLite handler
│   │   └── middleware.go     # HTTP request logging
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add logging configuration to CLAUDE.md"
```

---

## Task 10: Run Full Test Suite

**Step 1: Run all Go tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 2: Run E2E tests (if server running)**

Run: `cd e2e && npm test`
Expected: All tests pass (logging should not affect behavior)

**Step 3: Manual verification**

```bash
# Console text
./sblite serve --db test.db --log-level=debug

# Console JSON
./sblite serve --db test.db --log-format=json

# File logging
./sblite serve --db test.db --log-mode=file --log-file=test.log
cat test.log

# Database logging
./sblite serve --db test.db --log-mode=database --log-db=test-log.db --log-fields=request_id,extra
sqlite3 test-log.db "SELECT * FROM logs"
```

**Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address test failures from logging integration"
```

---

## Summary

Total tasks: 10
Estimated commits: 10-12

Key deliverables:
1. `internal/log/` package with 5 files
2. Console, file, and database handlers
3. HTTP request logging middleware
4. CLI flags and env vars in serve command
5. Documentation updates
