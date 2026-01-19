# Logging System

sblite includes a configurable logging system built on Go's `slog` package. It supports three output modes (console, file, database) with structured logging, automatic rotation, and retention policies.

## Quick Start

```bash
# Default: console logging with text format
./sblite serve

# JSON format for log aggregators (Datadog, Splunk, etc.)
./sblite serve --log-format=json

# File logging with automatic rotation
./sblite serve --log-mode=file --log-file=/var/log/sblite.log

# Database logging for queryable logs
./sblite serve --log-mode=database --log-db=/var/log/sblite-logs.db
```

## Logging Modes

### Console Mode (Default)

Writes logs to stdout. Best for development and containerized deployments where logs are captured by the container runtime.

**Text format (default):**
```
time=2024-01-17T12:00:00.000Z level=INFO msg="starting server" addr=0.0.0.0:8080
time=2024-01-17T12:00:01.000Z level=INFO msg="http request" method=GET path=/health status=200 duration_ms=1
```

**JSON format:**
```json
{"time":"2024-01-17T12:00:00.000Z","level":"INFO","msg":"starting server","addr":"0.0.0.0:8080"}
{"time":"2024-01-17T12:00:01.000Z","level":"INFO","msg":"http request","method":"GET","path":"/health","status":200,"duration_ms":1}
```

**Configuration:**
```bash
# Text format
./sblite serve --log-format=text

# JSON format
./sblite serve --log-format=json

# Set log level
./sblite serve --log-level=debug  # debug, info, warn, error
```

### File Mode

Writes logs to a file with automatic size-based rotation. Old files are cleaned up based on age and backup count limits.

**How rotation works:**
1. Logs write to the configured file (e.g., `sblite.log`)
2. When file exceeds `--log-max-size` MB, it rotates
3. Rotated files are renamed with timestamp: `sblite.log.2024-01-17T12-00-00`
4. Files older than `--log-max-age` days are deleted
5. Only `--log-max-backups` most recent backups are kept

**Configuration:**
```bash
./sblite serve \
  --log-mode=file \
  --log-file=/var/log/sblite.log \
  --log-format=json \
  --log-max-size=100 \      # Rotate at 100 MB
  --log-max-age=7 \         # Delete files older than 7 days
  --log-max-backups=3       # Keep at most 3 backup files
```

**Example directory after rotation:**
```
/var/log/
├── sblite.log                      # Current log file
├── sblite.log.2024-01-17T12-00-00  # Backup 1 (newest)
├── sblite.log.2024-01-16T12-00-00  # Backup 2
└── sblite.log.2024-01-15T12-00-00  # Backup 3 (oldest kept)
```

### Database Mode

Writes logs to a SQLite database for queryable, structured log storage. Includes automatic retention cleanup.

**Schema:**
```sql
CREATE TABLE logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,      -- ISO 8601 format
    level TEXT NOT NULL,          -- DEBUG, INFO, WARN, ERROR
    message TEXT NOT NULL,
    source TEXT,                  -- File:line (if enabled)
    request_id TEXT,              -- HTTP request correlation ID
    user_id TEXT,                 -- Authenticated user ID
    extra TEXT                    -- JSON blob for additional fields
);

CREATE INDEX idx_logs_timestamp ON logs(timestamp);
CREATE INDEX idx_logs_level ON logs(level);
```

**Configuration:**
```bash
./sblite serve \
  --log-mode=database \
  --log-db=/var/log/sblite-logs.db \
  --log-max-age=7 \                          # Delete entries older than 7 days
  --log-fields=source,request_id,user_id,extra
```

**Available fields (`--log-fields`):**
| Field | Description |
|-------|-------------|
| `source` | Source file and line number |
| `request_id` | HTTP request correlation ID |
| `user_id` | Authenticated user's ID |
| `extra` | JSON blob with additional log attributes |

**Querying logs:**
```bash
# Recent errors
sqlite3 /var/log/sblite-logs.db \
  "SELECT timestamp, message FROM logs WHERE level='ERROR' ORDER BY timestamp DESC LIMIT 10"

# Requests for a specific user
sqlite3 /var/log/sblite-logs.db \
  "SELECT timestamp, message, request_id FROM logs WHERE user_id='abc123'"

# Slow requests (requires extra field)
sqlite3 /var/log/sblite-logs.db \
  "SELECT * FROM logs WHERE json_extract(extra, '$.duration_ms') > 1000"

# Count by level in last hour
sqlite3 /var/log/sblite-logs.db \
  "SELECT level, COUNT(*) FROM logs WHERE timestamp > datetime('now', '-1 hour') GROUP BY level"
```

**Retention cleanup:**
- Runs automatically every hour
- Deletes entries older than `--log-max-age` days
- Uses the `timestamp` column for age calculation

## In-Memory Log Buffer

sblite maintains an in-memory ring buffer that captures all log output, regardless of the configured log mode. This allows you to view recent console logs from the dashboard even when running in the background.

**Configuration:**
```bash
# Default: 500 lines
./sblite serve

# Custom buffer size
./sblite serve --log-buffer-lines=1000

# Disable buffer (minimal memory footprint)
./sblite serve --log-buffer-lines=0
```

**Viewing logs in the dashboard:**
1. Open the dashboard at `http://localhost:8080/_`
2. Navigate to **Logs** in the sidebar
3. The **Console** tab shows buffered log output
4. Use **Auto-refresh (5s)** to automatically poll for new logs
5. Click **Refresh** for manual updates

**API endpoint:**
```bash
# Get last 100 lines (default)
curl http://localhost:8080/_/api/logs/buffer

# Get last 500 lines
curl http://localhost:8080/_/api/logs/buffer?lines=500
```

**Response format:**
```json
{
  "lines": ["time=... level=INFO msg=\"starting server\"", "..."],
  "total": 342,
  "showing": 100,
  "buffer_size": 500,
  "enabled": true
}
```

**When disabled:**
```json
{
  "lines": [],
  "total": 0,
  "showing": 0,
  "buffer_size": 0,
  "enabled": false,
  "message": "Log buffer is disabled. Start server with --log-buffer-lines=500"
}
```

**Notes:**
- The buffer works alongside all log modes (console, file, database)
- Buffer clears on server restart
- Typical memory usage: ~100KB for 500 lines

## Log Levels

| Level | Value | Use Case |
|-------|-------|----------|
| `debug` | -4 | Detailed debugging information |
| `info` | 0 | General operational messages (default) |
| `warn` | 4 | Warning conditions |
| `error` | 8 | Error conditions |

Messages below the configured level are filtered out. For example, `--log-level=warn` suppresses `debug` and `info` messages.

## HTTP Request Logging

All HTTP requests are automatically logged with:

| Field | Description |
|-------|-------------|
| `method` | HTTP method (GET, POST, etc.) |
| `path` | Request path |
| `status` | Response status code |
| `duration_ms` | Request duration in milliseconds |
| `request_id` | Unique 8-character request ID |
| `remote_addr` | Client IP address |

**Log level by status code:**
- 2xx, 3xx: `INFO`
- 4xx: `WARN`
- 5xx: `ERROR`

**Example output:**
```
level=INFO msg="http request" method=GET path=/auth/v1/user status=200 duration_ms=5 request_id=a1b2c3d4
level=WARN msg="http request" method=POST path=/auth/v1/token status=401 duration_ms=12 request_id=e5f6g7h8
level=ERROR msg="http request" method=GET path=/rest/v1/users status=500 duration_ms=3 request_id=i9j0k1l2
```

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_LOG_MODE` | `console` | Output mode: `console`, `file`, `database` |
| `SBLITE_LOG_LEVEL` | `info` | Minimum level: `debug`, `info`, `warn`, `error` |
| `SBLITE_LOG_FORMAT` | `text` | Output format: `text`, `json` |
| `SBLITE_LOG_FILE` | `sblite.log` | Log file path (file mode) |
| `SBLITE_LOG_DB` | `log.db` | Log database path (database mode) |
| `SBLITE_LOG_MAX_SIZE` | `100` | Max file size in MB before rotation |
| `SBLITE_LOG_MAX_AGE` | `7` | Days to retain old logs |
| `SBLITE_LOG_MAX_BACKUPS` | `3` | Backup files to keep (file mode) |
| `SBLITE_LOG_FIELDS` | `` | Database fields (comma-separated) |
| `SBLITE_LOG_BUFFER_LINES` | `500` | In-memory buffer size (0 to disable) |

### CLI Flags

All environment variables have corresponding CLI flags with `--log-` prefix:

```bash
./sblite serve \
  --log-mode=file \
  --log-level=debug \
  --log-format=json \
  --log-file=/var/log/sblite.log \
  --log-max-size=50 \
  --log-max-age=14 \
  --log-max-backups=5
```

**Priority:** CLI flags override environment variables.

## Use Case Examples

### Development

```bash
# Verbose console output
./sblite serve --log-level=debug
```

### Production with Log Aggregator

```bash
# JSON to stdout, captured by Docker/Kubernetes
./sblite serve --log-format=json --log-level=info
```

### Production with File Logging

```bash
# Rotate at 50MB, keep 7 days, max 10 backups
./sblite serve \
  --log-mode=file \
  --log-file=/var/log/sblite/app.log \
  --log-format=json \
  --log-max-size=50 \
  --log-max-age=7 \
  --log-max-backups=10
```

### Audit Logging

```bash
# Database with full context for compliance
./sblite serve \
  --log-mode=database \
  --log-db=/var/log/sblite-audit.db \
  --log-level=info \
  --log-max-age=90 \
  --log-fields=source,request_id,user_id,extra
```

### Debugging Production Issues

```bash
# Temporarily enable debug logging
SBLITE_LOG_LEVEL=debug ./sblite serve
```

## Architecture

```
┌─────────────────────────────────────────────┐
│              Application Code               │
│  log.Info("message", "key", "value")        │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│           slog.Logger (default)             │
│         Level filtering applied             │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│          Buffer Handler (optional)          │
│  Stores formatted lines in ring buffer      │
│  Exposes via /_/api/logs/buffer             │
└──────────────────┬──────────────────────────┘
                   │
        ┌──────────┼──────────┐
        ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Console  │ │   File   │ │ Database │
│ Handler  │ │ Handler  │ │ Handler  │
├──────────┤ ├──────────┤ ├──────────┤
│ text/json│ │ rotation │ │ retention│
│ to stdout│ │ cleanup  │ │ cleanup  │
└──────────┘ └──────────┘ └──────────┘
```

The logging system uses Go's `log/slog` package with custom handlers. The Buffer Handler wraps the primary handler (based on `--log-mode`) and captures all output in memory for dashboard viewing.
