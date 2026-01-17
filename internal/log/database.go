// Package log provides configurable logging for sblite.
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
