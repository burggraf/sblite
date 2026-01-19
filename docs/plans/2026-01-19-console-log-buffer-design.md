# Console Log Buffer for Dashboard

## Overview

Add an in-memory ring buffer that captures all log output, making recent console logs viewable in the dashboard regardless of the configured log mode.

## Problem

When sblite runs in the background with default console logging, there's no way to view logs (including email debug output) from the dashboard. Users must either:
- Run in foreground to see console output
- Switch to file or database logging mode

## Solution

A ring buffer that always captures log output alongside the primary log handler, with a dashboard UI to view the buffer contents.

## Architecture

### Ring Buffer Handler

New `internal/log/buffer.go`:
- Implements `slog.Handler` interface
- Wraps any existing handler (console, file, or database)
- Stores last N formatted log lines in a thread-safe circular buffer
- Exposes `Lines(n int) []string` method to retrieve buffered content

Data flow:
```
Log call → BufferHandler → stores in ring buffer
                        → forwards to wrapped handler (console/file/db)
```

### Configuration

**New flag:** `--log-buffer-lines` (default: 500)
**Environment variable:** `SBLITE_LOG_BUFFER_LINES`

Setting to 0 disables the buffer entirely.

Updates to `internal/log/Config`:
```go
type Config struct {
    // ... existing fields
    BufferLines int  // In-memory buffer size (0 to disable)
}
```

## API Endpoint

**Endpoint:** `GET /_/api/logs/buffer`

**Query parameters:**
- `lines` - Number of lines to return (default: 100, max: buffer size)

**Response:**
```json
{
  "lines": ["2024-01-19T10:15:32Z INFO server started", "..."],
  "total": 342,
  "showing": 100,
  "buffer_size": 500
}
```

## Dashboard UI

**Location:** Existing Logs section, new "Console" tab (shown first)

**Elements:**
- Tab selector: `[Console] [Database] [File]`
- Lines dropdown: 50, 100, 200, 500
- Refresh button (manual)
- Auto-refresh toggle: "Auto-refresh (5s)", off by default
- Monospace log display with auto-scroll

**Behavior:**
- Console tab always available
- Auto-refresh polls every 5 seconds when enabled
- Shows: "Showing 100 of 342 lines (buffer: 500)"

## Files Changed

**Create:**
- `internal/log/buffer.go` - Ring buffer handler
- `internal/log/buffer_test.go` - Unit tests

**Modify:**
- `internal/log/logger.go` - Add BufferLines config, wrap handler with buffer
- `cmd/serve.go` - Add `--log-buffer-lines` flag
- `internal/dashboard/handler.go` - Add `handleBufferLogs` endpoint
- `internal/dashboard/static/app.js` - Console tab UI with auto-refresh

## Implementation Notes

- Buffer uses `sync.RWMutex` for thread safety
- Lines stored as formatted strings (not structured) for simplicity
- No persistence - buffer clears on restart
- Memory usage: ~500 lines × ~200 bytes avg = ~100KB typical
