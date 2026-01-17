// Package log provides configurable logging for sblite with console, file, and database backends.
package log

import (
	"context"
	"fmt"
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

// Stub handlers - will be replaced in subsequent tasks

// NewFileHandler creates a file handler (stub - not implemented).
func NewFileHandler(cfg *Config, level slog.Level) (slog.Handler, error) {
	return nil, fmt.Errorf("file handler not implemented")
}

// NewDBHandler creates a database handler (stub - not implemented).
func NewDBHandler(cfg *Config, level slog.Level) (slog.Handler, error) {
	return nil, fmt.Errorf("database handler not implemented")
}
