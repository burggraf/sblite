// Package log provides configurable logging for sblite.
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
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
