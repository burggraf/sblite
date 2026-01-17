# Logging System Design

**Date:** 2026-01-17
**Status:** Draft

## Overview

Add configurable logging to sblite with support for console, file, and database outputs. The system uses Go's standard `log/slog` package with custom handlers for file rotation and SQLite storage.

## Requirements

- **Log levels:** debug, info, warn, error (standard 4-level)
- **Output modes:** console, file, or database (single active output)
- **Output formats:** text or JSON (configurable for console/file)
- **File rotation:** by size, with age-based cleanup and backup limits
- **Database retention:** automatic purging of old entries
- **Configurable schema:** choose which optional fields to store in database
- **HTTP integration:** replace Chi's middleware.Logger with unified logging
- **Configuration:** CLI flags + environment variables (consistent with existing patterns)

## Package Structure

```
internal/log/
├── logger.go      # Global logger, initialization, level filtering
├── handler.go     # Handler interface, base implementation
├── console.go     # Console handler (text/JSON format)
├── file.go        # File handler with rotation
├── database.go    # SQLite handler with retention
└── middleware.go  # HTTP request logging middleware for Chi
```

## Configuration

### Config Struct

```go
type Config struct {
    Mode     string // "console", "file", "database"
    Level    string // "debug", "info", "warn", "error"
    Format   string // "text", "json" (for console/file only)

    // File-specific
    FilePath    string
    MaxSizeMB   int  // Rotate when file exceeds this size
    MaxAgeDays  int  // Delete files older than this
    MaxBackups  int  // Keep at most this many old files

    // Database-specific
    DBPath         string   // Path to log.db
    RetentionDays  int      // Delete entries older than this
    Fields         []string // Optional fields: "source", "request_id", "user_id", "extra"
}
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--log-mode` | `console` | Output: console, file, database |
| `--log-level` | `info` | Level: debug, info, warn, error |
| `--log-format` | `text` | Format: text, json |
| `--log-file` | `sblite.log` | Log file path (file mode) |
| `--log-db` | `log.db` | Log database path (database mode) |
| `--log-max-size` | `100` | Max file size in MB before rotation |
| `--log-max-age` | `7` | Max age in days for retention |
| `--log-max-backups` | `3` | Max old log files to keep |
| `--log-fields` | `` | DB fields: source,request_id,user_id,extra |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_LOG_MODE` | `console` | Output mode |
| `SBLITE_LOG_LEVEL` | `info` | Minimum level |
| `SBLITE_LOG_FORMAT` | `text` | Output format |
| `SBLITE_LOG_FILE` | `sblite.log` | File path |
| `SBLITE_LOG_DB` | `log.db` | Database path |
| `SBLITE_LOG_MAX_SIZE` | `100` | MB before rotation |
| `SBLITE_LOG_MAX_AGE` | `7` | Days retention |
| `SBLITE_LOG_MAX_BACKUPS` | `3` | Old files kept |
| `SBLITE_LOG_FIELDS` | `` | Comma-separated fields |

Priority: CLI flags > environment variables > defaults

## Implementation Details

### slog Integration

```go
// In logger.go
var defaultLogger *slog.Logger

func Init(cfg *Config) error {
    var handler slog.Handler

    switch cfg.Mode {
    case "console":
        handler = NewConsoleHandler(os.Stdout, cfg)
    case "file":
        handler = NewFileHandler(cfg)
    case "database":
        handler = NewDBHandler(cfg)
    default:
        handler = NewConsoleHandler(os.Stdout, cfg)
    }

    defaultLogger = slog.New(handler)
    slog.SetDefault(defaultLogger)
    return nil
}

// Convenience functions
func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }
```

### File Handler with Rotation

```go
type FileHandler struct {
    mu        sync.Mutex
    file      *os.File
    path      string
    maxSize   int64    // bytes
    maxAge    int      // days
    maxBackup int
    size      int64    // current file size
    format    string   // "text" or "json"
    level     slog.Level
}

func (h *FileHandler) Handle(ctx context.Context, r slog.Record) error {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.size >= h.maxSize {
        h.rotate()
    }

    line := h.formatRecord(r)
    n, err := h.file.Write(line)
    h.size += int64(n)
    return err
}

func (h *FileHandler) rotate() {
    h.file.Close()
    timestamp := time.Now().Format("2006-01-02T15-04-05")
    os.Rename(h.path, h.path+"."+timestamp)
    h.cleanOldBackups()
    h.file, _ = os.Create(h.path)
    h.size = 0
}
```

### Database Handler

```go
type DBHandler struct {
    db            *sql.DB
    stmt          *sql.Stmt
    retention     int
    fields        []string
    level         slog.Level
    cleanupTicker *time.Ticker
}

// Schema
const createTableSQL = `
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

func (h *DBHandler) startCleanup() {
    h.cleanupTicker = time.NewTicker(1 * time.Hour)
    go func() {
        for range h.cleanupTicker.C {
            cutoff := time.Now().AddDate(0, 0, -h.retention)
            h.db.Exec("DELETE FROM logs WHERE timestamp < ?", cutoff.Format(time.RFC3339))
        }
    }()
}
```

### HTTP Request Logging Middleware

```go
func RequestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        reqID := uuid.NewString()[:8]

        ww := &responseWriter{ResponseWriter: w, status: 200}
        ctx := context.WithValue(r.Context(), RequestIDKey, reqID)

        next.ServeHTTP(ww, r.WithContext(ctx))

        duration := time.Since(start)
        level := slog.LevelInfo
        if ww.status >= 500 {
            level = slog.LevelError
        } else if ww.status >= 400 {
            level = slog.LevelWarn
        }

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
```

## Migration Plan

### Replace fmt.Printf calls

**cmd/serve.go:**
```go
// Before
fmt.Println("Warning: Using default JWT secret...")
fmt.Printf("Starting Supabase Lite on %s\n", addr)

// After
log.Warn("using default JWT secret, set SBLITE_JWT_SECRET in production")
log.Info("starting server", "addr", addr, "auth_api", authURL, "rest_api", restURL)
```

**internal/server/auth_handlers.go:**
```go
// Before
fmt.Printf("Failed to send magic link email: %v\n", err)

// After
log.Error("failed to send magic link email", "error", err)
```

### Replace Chi middleware

**internal/server/server.go:**
```go
// Before
s.router.Use(middleware.Logger)

// After
s.router.Use(log.RequestLogger)
```

## Implementation Order

1. Create `internal/log/logger.go` with Config struct and Init function
2. Create `internal/log/console.go` with text/JSON formatting
3. Create `internal/log/file.go` with rotation logic
4. Create `internal/log/database.go` with SQLite handler and retention
5. Create `internal/log/middleware.go` for HTTP request logging
6. Add CLI flags and env var handling to `cmd/serve.go`
7. Initialize logging early in serve command
8. Replace `fmt.Printf` calls throughout codebase
9. Replace `middleware.Logger` with `log.RequestLogger`
10. Add tests for each handler
11. Update CLAUDE.md with logging documentation

## Example Usage

```bash
# Console logging (default)
./sblite serve

# File logging with JSON format
./sblite serve --log-mode=file --log-format=json --log-file=/var/log/sblite.log

# Database logging with full context
./sblite serve --log-mode=database --log-db=/var/log/sblite-log.db \
  --log-fields=source,request_id,user_id,extra --log-level=debug

# Via environment variables
export SBLITE_LOG_MODE=file
export SBLITE_LOG_LEVEL=warn
./sblite serve
```
