# Console Log Buffer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an in-memory ring buffer that captures all log output, viewable in the dashboard regardless of log mode.

**Architecture:** A `BufferHandler` wraps the existing slog handler, storing formatted log lines in a thread-safe circular buffer. The dashboard exposes this via `/_/api/logs/buffer` with a new Console tab in the Logs view.

**Tech Stack:** Go slog.Handler, sync.RWMutex, vanilla JavaScript

---

## Task 1: Ring Buffer Handler

**Files:**
- Create: `internal/log/buffer.go`
- Test: `internal/log/buffer_test.go`

**Step 1: Write the failing test**

Create `internal/log/buffer_test.go`:

```go
package log

import (
	"log/slog"
	"testing"
)

func TestBufferHandler_StoresLines(t *testing.T) {
	buf := NewRingBuffer(10)
	h := NewBufferHandler(slog.NewTextHandler(nil, nil), buf)

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	lines := buf.Lines(10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] == "" {
		t.Error("expected non-empty line")
	}
}

func TestRingBuffer_Capacity(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Add("line1")
	buf.Add("line2")
	buf.Add("line3")
	buf.Add("line4") // should evict line1

	lines := buf.Lines(10)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line2" {
		t.Errorf("expected oldest line to be 'line2', got %q", lines[0])
	}
	if lines[2] != "line4" {
		t.Errorf("expected newest line to be 'line4', got %q", lines[2])
	}
}

func TestRingBuffer_LinesLimit(t *testing.T) {
	buf := NewRingBuffer(10)
	for i := 0; i < 5; i++ {
		buf.Add("line")
	}

	lines := buf.Lines(3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestRingBuffer_Total(t *testing.T) {
	buf := NewRingBuffer(3)
	buf.Add("line1")
	buf.Add("line2")

	if buf.Total() != 2 {
		t.Errorf("expected total 2, got %d", buf.Total())
	}
	if buf.Capacity() != 3 {
		t.Errorf("expected capacity 3, got %d", buf.Capacity())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log -run TestBuffer -v`
Expected: FAIL with "undefined: NewRingBuffer"

**Step 3: Write minimal implementation**

Create `internal/log/buffer.go`:

```go
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
	level   slog.Level
}

// NewBufferHandler creates a handler that stores logs in the buffer and forwards to wrapped.
func NewBufferHandler(wrapped slog.Handler, buffer *RingBuffer) *BufferHandler {
	return &BufferHandler{
		wrapped: wrapped,
		buffer:  buffer,
		level:   slog.LevelDebug, // capture all levels
	}
}

func (h *BufferHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Always enabled for buffer capture; wrapped handler does its own filtering
	return true
}

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

func (h *BufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BufferHandler{
		wrapped: h.wrapped.WithAttrs(attrs),
		buffer:  h.buffer,
		level:   h.level,
	}
}

func (h *BufferHandler) WithGroup(name string) slog.Handler {
	return &BufferHandler{
		wrapped: h.wrapped.WithGroup(name),
		buffer:  h.buffer,
		level:   h.level,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/log -run TestBuffer -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/log/buffer.go internal/log/buffer_test.go
git commit -m "feat(log): add ring buffer handler for in-memory log capture"
```

---

## Task 2: Integrate Buffer with Logger

**Files:**
- Modify: `internal/log/logger.go`
- Test: `internal/log/logger_test.go`

**Step 1: Write the failing test**

Add to `internal/log/logger_test.go`:

```go
func TestInit_CreatesBuffer(t *testing.T) {
	cfg := &Config{
		Mode:        "console",
		Level:       "info",
		BufferLines: 100,
	}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Log something
	Info("test buffer message")

	// Check buffer has content
	lines := GetBufferedLogs(10)
	if len(lines) == 0 {
		t.Error("expected buffered logs, got none")
	}
}

func TestInit_BufferDisabled(t *testing.T) {
	cfg := &Config{
		Mode:        "console",
		Level:       "info",
		BufferLines: 0, // disabled
	}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	lines := GetBufferedLogs(10)
	if lines != nil {
		t.Error("expected nil when buffer disabled")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/log -run TestInit_CreatesBuffer -v`
Expected: FAIL with "undefined: GetBufferedLogs" or field error

**Step 3: Modify logger.go**

Add `BufferLines` to Config struct (after line 27):

```go
	// Buffer-specific
	BufferLines int // In-memory buffer size (0 to disable)
```

Update `DefaultConfig()` to include default buffer size:

```go
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
		BufferLines:   500,
	}
}
```

Add global buffer variable (after line 65):

```go
var (
	defaultLogger *slog.Logger
	logBuffer     *RingBuffer
	mu            sync.RWMutex
)
```

Update `Init()` function to wrap handler with buffer:

```go
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

	// Wrap with buffer handler if enabled
	if cfg.BufferLines > 0 {
		logBuffer = NewRingBuffer(cfg.BufferLines)
		handler = NewBufferHandler(handler, logBuffer)
	} else {
		logBuffer = nil
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	return nil
}
```

Add `GetBufferedLogs()` function at end of file:

```go
// GetBufferedLogs returns the last n lines from the log buffer.
// Returns nil if buffer is disabled.
func GetBufferedLogs(n int) []string {
	mu.RLock()
	defer mu.RUnlock()
	if logBuffer == nil {
		return nil
	}
	return logBuffer.Lines(n)
}

// GetBufferStats returns buffer statistics.
// Returns (total, capacity, ok). ok is false if buffer disabled.
func GetBufferStats() (total int, capacity int, ok bool) {
	mu.RLock()
	defer mu.RUnlock()
	if logBuffer == nil {
		return 0, 0, false
	}
	return logBuffer.Total(), logBuffer.Capacity(), true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/log -run TestInit -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/log/logger.go internal/log/logger_test.go
git commit -m "feat(log): integrate buffer handler with logger init"
```

---

## Task 3: Add CLI Flag

**Files:**
- Modify: `cmd/serve.go`

**Step 1: Add flag in init()**

Add after line 381 (after `log-fields` flag):

```go
	serveCmd.Flags().Int("log-buffer-lines", 500, "Number of log lines to keep in memory buffer (0 to disable)")
```

**Step 2: Update buildLogConfig()**

Add environment variable handling (after line 253, after Fields handling):

```go
	if bufferLines := os.Getenv("SBLITE_LOG_BUFFER_LINES"); bufferLines != "" {
		if v, err := strconv.Atoi(bufferLines); err == nil {
			cfg.BufferLines = v
		}
	}
```

Add CLI flag override (after line 282, after Fields override):

```go
	if bufferLines, _ := cmd.Flags().GetInt("log-buffer-lines"); cmd.Flags().Changed("log-buffer-lines") {
		cfg.BufferLines = bufferLines
	}
```

**Step 3: Verify it compiles**

Run: `go build -o sblite .`
Expected: Build succeeds

**Step 4: Test flag works**

Run: `./sblite serve --help | grep buffer`
Expected: Shows `--log-buffer-lines` flag

**Step 5: Commit**

```bash
git add cmd/serve.go
git commit -m "feat(cli): add --log-buffer-lines flag"
```

---

## Task 4: Dashboard API Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add route**

In `RegisterRoutes()`, add new route after line 211 (after `/tail`):

```go
			r.Get("/buffer", h.handleBufferLogs)
```

**Step 2: Add handler**

Add after `handleTailLogs` function (around line 2914):

```go
func (h *Handler) handleBufferLogs(w http.ResponseWriter, r *http.Request) {
	linesStr := r.URL.Query().Get("lines")
	numLines := 100
	if n, err := strconv.Atoi(linesStr); err == nil && n > 0 {
		numLines = n
	}

	lines := log.GetBufferedLogs(numLines)
	total, capacity, ok := log.GetBufferStats()

	w.Header().Set("Content-Type", "application/json")

	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lines":       []string{},
			"total":       0,
			"showing":     0,
			"buffer_size": 0,
			"enabled":     false,
			"message":     "Log buffer is disabled. Start server with --log-buffer-lines=500",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"lines":       lines,
		"total":       total,
		"showing":     len(lines),
		"buffer_size": capacity,
		"enabled":     true,
	})
}
```

Add import for log package if not present:

```go
	"github.com/markb/sblite/internal/log"
```

**Step 3: Verify it compiles**

Run: `go build -o sblite .`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(dashboard): add /api/logs/buffer endpoint"
```

---

## Task 5: Dashboard UI - Console Tab

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add state for console logs**

Find `logs:` state object (around line 63) and add:

```javascript
        logs: {
            config: null,
            list: [],
            total: 0,
            page: 1,
            pageSize: 50,
            filters: { level: 'all', since: '', until: '', search: '', user_id: '', request_id: '' },
            expandedLog: null,
            tailLines: [],
            loading: false,
            // New console buffer state
            activeTab: 'console',  // 'console', 'database', 'file'
            consoleLines: [],
            consoleTotal: 0,
            consoleBufferSize: 0,
            consoleEnabled: false,
            autoRefresh: false,
            autoRefreshInterval: null,
        },
```

**Step 2: Update loadLogs() method**

Replace the `loadLogs()` method (around line 3560):

```javascript
    async loadLogs() {
        this.state.logs.loading = true;
        this.render();

        try {
            // Load log config
            const configRes = await fetch('/_/api/logs/config');
            if (configRes.ok) {
                this.state.logs.config = await configRes.json();
            }

            // Always load console buffer
            await this.loadConsoleBuffer();

            // Load mode-specific logs
            if (this.state.logs.activeTab === 'database' && this.state.logs.config?.mode === 'database') {
                await this.queryLogs();
            } else if (this.state.logs.activeTab === 'file' && this.state.logs.config?.mode === 'file') {
                await this.tailLogs();
            }
        } catch (e) {
            this.state.error = 'Failed to load logs';
        }

        this.state.logs.loading = false;
        this.render();
    },

    async loadConsoleBuffer() {
        try {
            const res = await fetch('/_/api/logs/buffer?lines=500');
            if (res.ok) {
                const data = await res.json();
                this.state.logs.consoleLines = data.lines || [];
                this.state.logs.consoleTotal = data.total || 0;
                this.state.logs.consoleBufferSize = data.buffer_size || 0;
                this.state.logs.consoleEnabled = data.enabled !== false;
            }
        } catch (e) {
            this.state.logs.consoleLines = [];
            this.state.logs.consoleEnabled = false;
        }
    },

    setLogsTab(tab) {
        this.state.logs.activeTab = tab;
        this.stopAutoRefresh();
        this.loadLogs();
    },

    toggleAutoRefresh() {
        if (this.state.logs.autoRefresh) {
            this.stopAutoRefresh();
        } else {
            this.startAutoRefresh();
        }
        this.render();
    },

    startAutoRefresh() {
        this.state.logs.autoRefresh = true;
        this.state.logs.autoRefreshInterval = setInterval(() => {
            this.loadConsoleBuffer().then(() => this.render());
        }, 5000);
    },

    stopAutoRefresh() {
        this.state.logs.autoRefresh = false;
        if (this.state.logs.autoRefreshInterval) {
            clearInterval(this.state.logs.autoRefreshInterval);
            this.state.logs.autoRefreshInterval = null;
        }
    },
```

**Step 3: Update renderLogsView()**

Replace `renderLogsView()` method (around line 3659):

```javascript
    renderLogsView() {
        const { config, loading, activeTab } = this.state.logs;

        if (loading) {
            return '<div class="loading">Loading logs...</div>';
        }

        const dbEnabled = config?.mode === 'database';
        const fileEnabled = config?.mode === 'file';

        return `
            <div class="card-title">Logs</div>
            <div class="logs-view">
                <div class="logs-tabs">
                    <button class="tab-btn ${activeTab === 'console' ? 'active' : ''}"
                            onclick="App.setLogsTab('console')">Console</button>
                    <button class="tab-btn ${activeTab === 'database' ? 'active' : ''} ${!dbEnabled ? 'disabled' : ''}"
                            onclick="App.setLogsTab('database')" ${!dbEnabled ? 'disabled' : ''}>Database</button>
                    <button class="tab-btn ${activeTab === 'file' ? 'active' : ''} ${!fileEnabled ? 'disabled' : ''}"
                            onclick="App.setLogsTab('file')" ${!fileEnabled ? 'disabled' : ''}>File</button>
                </div>
                ${activeTab === 'console' ? this.renderConsoleLogs() :
                  activeTab === 'database' ? this.renderDatabaseLogs() :
                  this.renderFileLogs()}
            </div>
        `;
    },

    renderConsoleLogs() {
        const { consoleLines, consoleTotal, consoleBufferSize, consoleEnabled, autoRefresh } = this.state.logs;

        if (!consoleEnabled) {
            return `
                <div class="message message-info">
                    <p>Console log buffer is disabled.</p>
                    <p>Start server with <code>--log-buffer-lines=500</code> to enable.</p>
                </div>
            `;
        }

        return `
            <div class="logs-toolbar">
                <div class="logs-info">
                    Showing ${consoleLines.length} of ${consoleTotal} lines (buffer: ${consoleBufferSize})
                </div>
                <div class="logs-actions">
                    <label class="auto-refresh-label">
                        <input type="checkbox" ${autoRefresh ? 'checked' : ''}
                               onchange="App.toggleAutoRefresh()">
                        Auto-refresh (5s)
                    </label>
                    <button class="btn btn-secondary btn-sm" onclick="App.loadConsoleBuffer().then(() => App.render())">
                        Refresh
                    </button>
                </div>
            </div>
            <div class="console-output">
                <pre>${consoleLines.map(l => this.escapeHtml(l)).join('')}</pre>
            </div>
        `;
    },

    renderFileLogs() {
        const { config, tailLines } = this.state.logs;

        if (config?.mode !== 'file') {
            return `
                <div class="message message-info">
                    <p>File logging is not enabled.</p>
                    <p>Start server with <code>--log-mode=file</code> to enable.</p>
                </div>
            `;
        }

        return `
            <div class="logs-toolbar">
                <div class="logs-info">
                    Log file: ${config.file_path}
                </div>
                <div class="logs-actions">
                    <button class="btn btn-secondary btn-sm" onclick="App.tailLogs().then(() => App.render())">
                        Refresh
                    </button>
                </div>
            </div>
            <div class="console-output">
                <pre>${tailLines.map(l => this.escapeHtml(l)).join('\n')}</pre>
            </div>
        `;
    },
```

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add Console tab to logs view with auto-refresh"
```

---

## Task 6: Dashboard UI - Styling

**Files:**
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add tab and console styles**

Add at end of file:

```css
/* Logs tabs */
.logs-tabs {
    display: flex;
    gap: 0;
    margin-bottom: 1rem;
    border-bottom: 1px solid var(--border);
}

.tab-btn {
    padding: 0.5rem 1rem;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    cursor: pointer;
    color: var(--text-muted);
    font-size: 0.9rem;
}

.tab-btn:hover:not(:disabled) {
    color: var(--text);
}

.tab-btn.active {
    color: var(--primary);
    border-bottom-color: var(--primary);
}

.tab-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

/* Console output */
.console-output {
    background: var(--bg-dark, #1a1a2e);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 1rem;
    max-height: 500px;
    overflow: auto;
}

.console-output pre {
    margin: 0;
    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    font-size: 0.85rem;
    line-height: 1.4;
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text-light, #e0e0e0);
}

/* Auto-refresh label */
.auto-refresh-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
    cursor: pointer;
}

.auto-refresh-label input {
    cursor: pointer;
}

/* Logs toolbar improvements */
.logs-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
    flex-wrap: wrap;
    gap: 0.5rem;
}

.logs-info {
    color: var(--text-muted);
    font-size: 0.9rem;
}

.logs-actions {
    display: flex;
    align-items: center;
    gap: 1rem;
}
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/style.css
git commit -m "feat(dashboard): add styling for logs tabs and console output"
```

---

## Task 7: Update Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update environment variables table**

Add to the Logging Configuration table:

```markdown
| `--log-buffer-lines` | `SBLITE_LOG_BUFFER_LINES` | `500` | In-memory log buffer size (0 to disable) |
```

**Step 2: Update dashboard endpoints table**

Add to Dashboard API endpoints:

```markdown
| `/_/api/logs/buffer` | GET | Get buffered console logs |
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add log buffer configuration to CLAUDE.md"
```

---

## Task 8: Integration Test

**Step 1: Manual test**

```bash
# Build and start server
go build -o sblite . && ./sblite init --db test.db && ./sblite serve --db test.db

# In another terminal, verify endpoint works
curl http://localhost:8080/_/api/logs/buffer | jq .
```

Expected: JSON response with `lines`, `total`, `buffer_size`, `enabled: true`

**Step 2: Test dashboard UI**

1. Open http://localhost:8080/_
2. Login to dashboard
3. Navigate to Logs
4. Verify Console tab is active and shows recent logs
5. Toggle auto-refresh and verify it updates every 5 seconds
6. Click Database/File tabs and verify they show appropriate messages

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat: complete console log buffer implementation"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Ring buffer handler | `internal/log/buffer.go`, `buffer_test.go` |
| 2 | Logger integration | `internal/log/logger.go`, `logger_test.go` |
| 3 | CLI flag | `cmd/serve.go` |
| 4 | Dashboard API | `internal/dashboard/handler.go` |
| 5 | Dashboard UI | `internal/dashboard/static/app.js` |
| 6 | Styling | `internal/dashboard/static/style.css` |
| 7 | Documentation | `CLAUDE.md` |
| 8 | Integration test | Manual verification |
