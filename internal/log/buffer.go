// Package log provides configurable logging for sblite.
package log

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
)

// RingBuffer is a thread-safe circular buffer for log lines.
type RingBuffer struct {
	mu       sync.RWMutex
	lines    []string
	capacity int
	head     int  // next write position
	full     bool // buffer has wrapped
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 500
	}
	return &RingBuffer{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

// Add adds a line to the buffer, evicting the oldest if full.
func (rb *RingBuffer) Add(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.lines[rb.head] = line
	rb.head = (rb.head + 1) % rb.capacity
	if rb.head == 0 {
		rb.full = true
	}
}

// Lines returns the last n lines (oldest first).
func (rb *RingBuffer) Lines(n int) []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	total := rb.Total()
	if n > total {
		n = total
	}
	if n <= 0 {
		return []string{}
	}

	result := make([]string, n)
	start := 0
	if rb.full {
		start = rb.head
	}

	// Skip to get only last n lines
	skip := total - n
	for i := 0; i < n; i++ {
		idx := (start + skip + i) % rb.capacity
		result[i] = rb.lines[idx]
	}
	return result
}

// Total returns the number of lines currently in the buffer.
func (rb *RingBuffer) Total() int {
	if rb.full {
		return rb.capacity
	}
	return rb.head
}

// Capacity returns the buffer capacity.
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}

// BufferHandler wraps another handler and stores formatted logs in a ring buffer.
type BufferHandler struct {
	wrapped slog.Handler
	buffer  *RingBuffer
}

// NewBufferHandler creates a handler that stores logs in the buffer and forwards to wrapped.
func NewBufferHandler(wrapped slog.Handler, buffer *RingBuffer) *BufferHandler {
	return &BufferHandler{
		wrapped: wrapped,
		buffer:  buffer,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *BufferHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Always enabled for buffer capture; wrapped handler does its own filtering
	return true
}

// Handle writes the record to the buffer and forwards to the wrapped handler.
func (h *BufferHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format the record as text for the buffer
	var buf bytes.Buffer
	textHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	if err := textHandler.Handle(ctx, r); err == nil {
		h.buffer.Add(buf.String())
	}

	// Forward to wrapped handler if it accepts this level
	if h.wrapped != nil && h.wrapped.Enabled(ctx, r.Level) {
		return h.wrapped.Handle(ctx, r)
	}
	return nil
}

// WithAttrs returns a new handler with the given attributes.
func (h *BufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var wrapped slog.Handler
	if h.wrapped != nil {
		wrapped = h.wrapped.WithAttrs(attrs)
	}
	return &BufferHandler{
		wrapped: wrapped,
		buffer:  h.buffer,
	}
}

// WithGroup returns a new handler with the given group.
func (h *BufferHandler) WithGroup(name string) slog.Handler {
	var wrapped slog.Handler
	if h.wrapped != nil {
		wrapped = h.wrapped.WithGroup(name)
	}
	return &BufferHandler{
		wrapped: wrapped,
		buffer:  h.buffer,
	}
}
