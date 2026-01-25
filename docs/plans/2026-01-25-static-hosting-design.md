# Static File Hosting Design

## Overview

Add static file hosting to sblite on the same port as the API, serving files from a configurable directory with SPA fallback support.

## Configuration

- `--static-dir` flag with default `./public`
- `SBLITE_STATIC_DIR` environment variable (flag overrides env)
- Always enabled - if directory exists, serve files; if not, silently skip

## Behavior

- Static files served from root (`/`)
- API routes (`/auth/v1/*`, `/rest/v1/*`, etc.) take priority
- SPA fallback: unmatched routes return `index.html` for client-side routing
- Proper MIME type detection based on file extension

## Route Priority

Routes are matched in order:

1. `/health` - health check
2. `/auth/v1/*` - auth API
3. `/rest/v1/*` - REST API
4. `/storage/v1/*` - storage API
5. `/functions/v1/*` - edge functions
6. `/admin/v1/*` - admin API
7. `/_/*` - dashboard
8. `/realtime/v1/*` - WebSocket
9. `/*` - static files (catch-all, registered last)

## Implementation

### Files Modified

1. `cmd/serve.go` - Add flag, read env var, pass to server config
2. `internal/server/server.go` - Add static dir to config, register catch-all route

### Static File Handler Logic

```
Request comes in → Check if file exists in static dir
  ├─ File exists → Serve with correct MIME type
  ├─ Path has extension but file missing → 404
  └─ Path has no extension (e.g., /about) → Serve index.html (SPA fallback)
```

### Key Details

- Use Go's `http.FileServer` with custom wrapper for SPA fallback
- Remove the global `Content-Type: application/json` header for static routes
- Check directory existence at startup, log info if serving, skip silently if not
