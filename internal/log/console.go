// Package log provides configurable logging for sblite.
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
