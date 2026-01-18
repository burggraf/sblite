# sblite Edge Functions Design

**Date:** 2026-01-18
**Status:** Proposed
**Priority:** Medium (Major Feature)

## Overview

This document outlines the design for implementing Supabase-compatible Edge Functions in sblite. The goal is 100% compatibility with `@supabase/supabase-js` `functions.invoke()` while maintaining sblite's single-binary philosophy where possible.

## Research Summary

### Supabase Edge Functions Architecture

**Sources:**
- [Supabase Edge Functions Architecture](https://supabase.com/docs/guides/functions/architecture)
- [Supabase Edge Runtime GitHub](https://github.com/supabase/edge-runtime)
- [Edge Runtime Self-Hosted Blog](https://supabase.com/blog/edge-runtime-self-hosted-deno-functions)
- [functions.invoke() API Reference](https://supabase.com/docs/reference/javascript/functions-invoke)
- [Deno Edge Functions](https://supabase.com/features/deno-edge-functions)

**Key Findings:**

1. **Runtime Architecture:**
   - Supabase Edge Functions run on Deno runtime (V8 isolates)
   - Each invocation spins up a new V8 isolate for isolation
   - Two-layer runtime: Main Worker (request handling) + User Workers (function execution)
   - Built in Rust using `deno_core` for V8 abstraction
   - Sub-100ms cold starts due to ESZip bundling format

2. **Edge Runtime is Open Source:**
   - Full source available at [github.com/supabase/edge-runtime](https://github.com/supabase/edge-runtime)
   - Explicitly designed for self-hosting
   - Pre-built Docker images available
   - MIT licensed

3. **Function Loading:**
   - Functions bundled as ESZip (Deno's compact module graph format)
   - Loaded via `EdgeRuntime.userWorkers.create()` with configuration:
     - `servicePath`: Function module location
     - `memoryLimitMb`: Memory constraints (default 150MB)
     - `workerTimeoutMs`: Execution limits (default 60 seconds)
     - `importMapPath`: Dependency resolution
     - `envVars`: Environment variables

4. **HTTP Invocation Protocol:**
   - Endpoint: `POST /functions/v1/{function-name}`
   - Methods supported: GET, POST, PUT, PATCH, DELETE, OPTIONS
   - Authorization: JWT in `Authorization: Bearer <token>` header
   - Body: JSON, FormData, Blob, ArrayBuffer, or plain text
   - Response: Auto-parsed based on Content-Type (json, blob, form-data, text)

5. **SDK Integration:**
   ```typescript
   const { data, error } = await supabase.functions.invoke('my-function', {
     body: { name: 'World' },
     headers: { 'Custom-Header': 'value' },
     method: 'POST',  // default
   })
   ```

6. **Error Types:**
   - `FunctionsHttpError`: Function returned an error
   - `FunctionsRelayError`: Infrastructure relay error
   - `FunctionsFetchError`: Network/fetch error

### Constraints & Limitations

1. **Runtime Requirements:**
   - Edge Runtime requires Deno (V8-based JavaScript runtime)
   - Cannot be reimplemented in pure Go without massive effort
   - Must use or embed the actual Deno/edge-runtime

2. **Platform Support:**
   - Edge Runtime has binaries for: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
   - Windows support is less mature
   - Docker images available for containerized deployment

3. **Resource Limits:**
   - Memory: 150MB per function (configurable)
   - Duration: 60 seconds maximum (configurable)
   - User Workers lack host environment access by design

---

## Design Goals

1. **100% Supabase Client Compatibility** - Work with `supabase.functions.invoke()`
2. **Separate Executable** - Keep sblite Go-only, edge-runtime as companion
3. **Auto-Management** - sblite manages edge-runtime lifecycle automatically
4. **Development Experience** - Hot reload, local testing, logs
5. **Migration Path** - Functions deployable to Supabase without changes

---

## Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────────────────┐
│                         sblite serve                             │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Chi Router                              │   │
│  │  /auth/v1/*  →  Auth Handlers                             │   │
│  │  /rest/v1/*  →  REST Handlers                             │   │
│  │  /functions/v1/* → Functions Proxy ─────────────────────┐ │   │
│  └──────────────────────────────────────────────────────────┘ │   │
│                                                          │       │
└──────────────────────────────────────────────────────────│───────┘
                                                           │
                                                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Supabase Edge Runtime                         │
│                   (Separate Process/Binary)                      │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐        │
│  │  Main Worker  │  │ User Worker 1 │  │ User Worker 2 │  ...   │
│  │ (HTTP Proxy)  │  │ (my-func)     │  │ (hello)       │        │
│  └───────────────┘  └───────────────┘  └───────────────┘        │
└─────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                     ./functions/                                 │
│  ├── my-func/                                                   │
│  │   └── index.ts                                               │
│  ├── hello/                                                     │
│  │   └── index.ts                                               │
│  └── _shared/                                                   │
│      └── utils.ts                                               │
└─────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| sblite (Go) | HTTP routing, JWT validation, proxy to edge-runtime, process management |
| Edge Runtime (Rust/Deno) | Function execution, V8 isolates, Deno APIs, import resolution |
| Functions Directory | TypeScript/JavaScript source files, import maps, shared code |

---

## Implementation Strategy

### Option A: Auto-Download Edge Runtime (Recommended)

sblite automatically downloads the appropriate edge-runtime binary on first use.

**Pros:**
- Single `sblite` binary for distribution
- Automatic platform detection
- User doesn't need to install anything extra

**Cons:**
- First-run delay for download (~50MB)
- Requires network access initially
- Need to handle binary updates

### Option B: Bundled Edge Runtime

Ship edge-runtime alongside sblite in releases.

**Pros:**
- Works offline immediately
- Predictable versions

**Cons:**
- Larger download (~50MB+ per platform)
- More complex release process
- Cross-platform builds more complex

### Option C: Require Separate Install

User installs edge-runtime separately (via Docker or binary).

**Pros:**
- Simplest for sblite
- User controls edge-runtime version

**Cons:**
- Worse user experience
- More setup steps
- Version compatibility issues

**Recommendation:** Option A (auto-download) with Option C as fallback for advanced users.

---

## Package Structure

```
sblite/
├── cmd/
│   └── functions.go              # CLI: new, serve, list, deploy
├── internal/
│   └── functions/
│       ├── functions.go          # Main service, orchestration
│       ├── runtime.go            # Edge runtime process management
│       ├── download.go           # Runtime binary download/update
│       ├── proxy.go              # HTTP proxy to edge runtime
│       ├── handler.go            # HTTP handlers for /functions/v1/*
│       ├── scaffold.go           # Function scaffolding templates
│       └── functions_test.go     # Tests
├── functions/                    # Default functions directory
│   └── .gitkeep
└── bin/                          # Downloaded binaries (gitignored)
    └── edge-runtime-{os}-{arch}
```

---

## Database Schema

### functions_config Table

```sql
CREATE TABLE IF NOT EXISTS functions_config (
    key           TEXT PRIMARY KEY,
    value         TEXT NOT NULL,
    updated_at    TEXT DEFAULT (datetime('now'))
);

-- Keys:
-- 'functions_dir' = Path to functions directory (default: ./functions)
-- 'runtime_path' = Path to edge-runtime binary (default: auto-download)
-- 'runtime_port' = Internal port for edge-runtime (default: 8081)
-- 'default_memory_mb' = Default memory limit per function (default: 150)
-- 'default_timeout_ms' = Default timeout per function (default: 60000)
-- 'verify_jwt' = Whether to verify JWT by default (default: '1')
```

### functions_secrets Table

```sql
CREATE TABLE IF NOT EXISTS functions_secrets (
    name          TEXT PRIMARY KEY,
    value         TEXT NOT NULL,  -- Encrypted with JWT secret
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);
```

### functions_metadata Table (Optional - for function-specific config)

```sql
CREATE TABLE IF NOT EXISTS functions_metadata (
    name          TEXT PRIMARY KEY,
    verify_jwt    INTEGER DEFAULT 1,
    memory_mb     INTEGER,
    timeout_ms    INTEGER,
    import_map    TEXT,  -- Path to custom import_map.json
    env_vars      TEXT DEFAULT '{}' CHECK (json_valid(env_vars)),
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);
```

---

## REST API Endpoints

### Function Invocation

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/functions/v1/{name}` | Invoke function (GET) | JWT (configurable) |
| POST | `/functions/v1/{name}` | Invoke function (POST) | JWT (configurable) |
| PUT | `/functions/v1/{name}` | Invoke function (PUT) | JWT (configurable) |
| PATCH | `/functions/v1/{name}` | Invoke function (PATCH) | JWT (configurable) |
| DELETE | `/functions/v1/{name}` | Invoke function (DELETE) | JWT (configurable) |
| OPTIONS | `/functions/v1/{name}` | CORS preflight | None |

### Request Headers (Passed to Function)

| Header | Description |
|--------|-------------|
| `Authorization` | Bearer JWT token |
| `Content-Type` | Request body type |
| `X-Forwarded-For` | Original client IP |
| `X-Request-Id` | Unique request identifier |
| All custom headers | Passed through to function |

### Response Headers (From Function)

| Header | Description |
|--------|-------------|
| `Content-Type` | Response body type |
| `X-Deno-Subhost` | Edge runtime identifier |
| All custom headers | Passed through from function |

### Dashboard Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/_/api/functions` | List all functions |
| GET | `/_/api/functions/{name}` | Get function details |
| POST | `/_/api/functions/{name}` | Create/update function |
| DELETE | `/_/api/functions/{name}` | Delete function |
| GET | `/_/api/functions/{name}/logs` | Get function logs |
| GET | `/_/api/functions/secrets` | List secrets (names only) |
| POST | `/_/api/functions/secrets` | Create/update secret |
| DELETE | `/_/api/functions/secrets/{name}` | Delete secret |
| GET | `/_/api/functions/config` | Get functions config |
| PATCH | `/_/api/functions/config` | Update functions config |

---

## Runtime Management

### Process Lifecycle

```go
// internal/functions/runtime.go
package functions

type RuntimeManager struct {
    binaryPath   string
    functionsDir string
    port         int
    process      *os.Process
    healthCheck  *time.Ticker
    mu           sync.Mutex
}

// Start launches the edge runtime process
func (rm *RuntimeManager) Start(ctx context.Context) error {
    // 1. Ensure binary exists (download if needed)
    // 2. Build command with environment
    // 3. Start process
    // 4. Wait for health check
    // 5. Start health monitor goroutine
}

// Stop gracefully shuts down the runtime
func (rm *RuntimeManager) Stop() error {
    // 1. Send SIGTERM
    // 2. Wait with timeout
    // 3. SIGKILL if needed
}

// Restart performs graceful restart
func (rm *RuntimeManager) Restart() error {
    rm.Stop()
    return rm.Start(context.Background())
}

// ensureBinary downloads edge-runtime if not present
func (rm *RuntimeManager) ensureBinary() error {
    // 1. Check if binary exists at rm.binaryPath
    // 2. Verify checksum if exists
    // 3. Download appropriate version for OS/arch if needed
    // 4. Make executable
}
```

### Environment Variables Passed to Runtime

```go
env := []string{
    fmt.Sprintf("SUPABASE_URL=http://localhost:%d", sblitePort),
    fmt.Sprintf("SUPABASE_ANON_KEY=%s", anonKey),
    fmt.Sprintf("SUPABASE_SERVICE_ROLE_KEY=%s", serviceKey),
    fmt.Sprintf("SUPABASE_DB_URL=sqlite://%s", dbPath),
    // User-defined secrets from functions_secrets table
}
```

---

## Proxy Implementation

```go
// internal/functions/proxy.go
package functions

import (
    "net/http"
    "net/http/httputil"
    "net/url"
)

type FunctionsProxy struct {
    runtimeURL *url.URL
    proxy      *httputil.ReverseProxy
    jwtSecret  []byte
}

func NewFunctionsProxy(runtimePort int, jwtSecret []byte) *FunctionsProxy {
    target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", runtimePort))

    proxy := httputil.NewSingleHostReverseProxy(target)
    proxy.ModifyResponse = modifyResponse
    proxy.ErrorHandler = errorHandler

    return &FunctionsProxy{
        runtimeURL: target,
        proxy:      proxy,
        jwtSecret:  jwtSecret,
    }
}

func (fp *FunctionsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Extract function name from path
    // 2. Check if JWT verification required for this function
    // 3. Validate JWT if required
    // 4. Add X-Forwarded headers
    // 5. Add X-Request-Id if not present
    // 6. Proxy to edge runtime
    fp.proxy.ServeHTTP(w, r)
}

func modifyResponse(resp *http.Response) error {
    // Add any response modifications
    // e.g., CORS headers if needed
    return nil
}

func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
    // Return FunctionsRelayError format
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadGateway)
    json.NewEncoder(w).Encode(map[string]string{
        "error": "FunctionsRelayError",
        "message": err.Error(),
    })
}
```

---

## CLI Commands

### Function Management

```bash
# Create new function from template
./sblite functions new hello-world
# Creates ./functions/hello-world/index.ts

# List all functions
./sblite functions list
# Output:
# NAME          STATUS    VERIFY_JWT
# hello-world   ready     true
# my-func       ready     false

# Start local development server (with hot reload)
./sblite functions serve
# Starts edge-runtime watching ./functions directory

# Start sblite with functions enabled
./sblite serve --functions
# Starts both sblite and edge-runtime

# Get function logs
./sblite functions logs hello-world
./sblite functions logs hello-world --follow

# Delete function
./sblite functions delete hello-world
```

### Secrets Management

```bash
# Set a secret
./sblite functions secrets set MY_API_KEY
# Prompts for value (hidden input)

# Set from value
./sblite functions secrets set MY_API_KEY=sk-123456

# List secrets (names only)
./sblite functions secrets list
# Output:
# MY_API_KEY
# DATABASE_URL

# Delete a secret
./sblite functions secrets delete MY_API_KEY
```

### Configuration

```bash
# View functions config
./sblite functions config

# Set functions directory
./sblite functions config --dir /path/to/functions

# Set custom runtime path (skip auto-download)
./sblite functions config --runtime /path/to/edge-runtime

# Set default memory/timeout
./sblite functions config --memory 256 --timeout 120000
```

---

## Function Template

### Default Scaffold

```typescript
// functions/hello-world/index.ts
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

serve(async (req: Request) => {
  const { name } = await req.json()

  const data = {
    message: `Hello ${name}!`,
  }

  return new Response(
    JSON.stringify(data),
    { headers: { "Content-Type": "application/json" } },
  )
})
```

### With Supabase Client

```typescript
// functions/get-user/index.ts
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from "https://esm.sh/@supabase/supabase-js@2"

serve(async (req: Request) => {
  // Create Supabase client using auto-injected env vars
  const supabase = createClient(
    Deno.env.get('SUPABASE_URL') ?? '',
    Deno.env.get('SUPABASE_ANON_KEY') ?? '',
    {
      global: {
        headers: { Authorization: req.headers.get('Authorization')! },
      },
    }
  )

  // Get authenticated user
  const { data: { user }, error } = await supabase.auth.getUser()

  if (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 401,
      headers: { "Content-Type": "application/json" },
    })
  }

  return new Response(JSON.stringify({ user }), {
    headers: { "Content-Type": "application/json" },
  })
})
```

---

## Binary Management

### Download URLs

```go
const (
    edgeRuntimeVersion = "v1.67.4"  // Pin to specific version
    baseURL = "https://github.com/supabase/edge-runtime/releases/download"
)

func getBinaryURL(version, goos, goarch string) string {
    // Map Go arch to edge-runtime naming
    arch := goarch
    if goarch == "amd64" {
        arch = "x86_64"
    } else if goarch == "arm64" {
        arch = "aarch64"
    }

    // Map Go OS to edge-runtime naming
    os := goos
    if goos == "darwin" {
        os = "apple-darwin"
    } else if goos == "linux" {
        os = "unknown-linux-gnu"
    }

    filename := fmt.Sprintf("edge-runtime_%s_%s-%s.tar.gz", version, arch, os)
    return fmt.Sprintf("%s/%s/%s", baseURL, version, filename)
}
```

### Checksum Verification

```go
// SHA256 checksums for each platform binary
var checksums = map[string]string{
    "darwin-amd64":  "abc123...",
    "darwin-arm64":  "def456...",
    "linux-amd64":   "ghi789...",
    "linux-arm64":   "jkl012...",
}

func verifyChecksum(binaryPath, expected string) error {
    f, _ := os.Open(binaryPath)
    defer f.Close()

    h := sha256.New()
    io.Copy(h, f)
    actual := hex.EncodeToString(h.Sum(nil))

    if actual != expected {
        return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
    }
    return nil
}
```

---

## Dashboard Integration

### Functions Tab

- **Function List:** Name, status, last invoked, JWT verification toggle
- **Function Editor:** Monaco editor for TypeScript with Deno type hints
- **Logs Viewer:** Real-time logs from edge-runtime
- **Secrets Manager:** Add/edit/delete secrets (values hidden)
- **Test Console:** Invoke functions with custom body/headers

### UI Mockup

```
┌──────────────────────────────────────────────────────────────┐
│  Functions                                    [+ New Function]│
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐ │
│  │ NAME          │ STATUS │ VERIFY JWT │ LAST INVOKED      │ │
│  ├───────────────┼────────┼────────────┼───────────────────┤ │
│  │ hello-world   │ ● Ready│ ✓          │ 2 minutes ago     │ │
│  │ process-order │ ● Ready│ ✓          │ 15 minutes ago    │ │
│  │ webhook       │ ● Ready│ ✗          │ 1 hour ago        │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
│  [Secrets] [Configuration] [Logs]                            │
└──────────────────────────────────────────────────────────────┘
```

---

## Error Handling

### Error Response Format

Match Supabase's error format for SDK compatibility:

```typescript
// FunctionsHttpError - Function returned an error
{
  "error": "FunctionsHttpError",
  "message": "Function returned an error",
  "context": { /* original response body */ }
}

// FunctionsRelayError - Infrastructure error
{
  "error": "FunctionsRelayError",
  "message": "Edge runtime unavailable"
}

// FunctionsFetchError - Network error (handled by SDK)
```

### Error Scenarios

| Scenario | HTTP Status | Error Type |
|----------|-------------|------------|
| Function not found | 404 | FunctionsHttpError |
| JWT invalid/missing | 401 | FunctionsHttpError |
| Function threw error | 500 | FunctionsHttpError |
| Edge runtime down | 502 | FunctionsRelayError |
| Function timeout | 504 | FunctionsRelayError |
| Request too large | 413 | FunctionsHttpError |

---

## Hot Reload (Development)

Edge runtime supports file watching for development:

```bash
# Development mode with hot reload
./sblite functions serve --watch

# Or as part of main server
./sblite serve --functions --functions-watch
```

When files change:
1. Edge runtime detects change via filesystem watcher
2. Affected user worker is terminated
3. Next request spawns new worker with updated code
4. No full restart needed

---

## Implementation Phases

### Phase 1: Core Integration
1. Runtime manager (start/stop/health check)
2. Binary download and verification
3. HTTP proxy to edge runtime
4. Basic `/functions/v1/{name}` routing
5. CLI: `functions serve`, `functions new`

**Estimated scope:** ~600-800 lines of Go

### Phase 2: Configuration & Secrets
1. Database tables (config, secrets, metadata)
2. Environment variable injection
3. Per-function JWT verification toggle
4. CLI: `functions secrets`, `functions config`

**Estimated scope:** ~400-500 lines of Go

### Phase 3: Dashboard UI
1. Functions list view
2. Function editor (basic)
3. Secrets management UI
4. Logs viewer
5. Test console

**Estimated scope:** ~800-1000 lines (Go + JS)

### Phase 4: Polish & Testing
1. E2E tests with `@supabase/supabase-js`
2. Error handling refinement
3. Documentation
4. CLAUDE.md updates

**Estimated scope:** ~300-400 lines + tests

---

## Platform Support Matrix

| Platform | Support Level | Notes |
|----------|---------------|-------|
| linux/amd64 | Full | Primary target |
| linux/arm64 | Full | Docker, Raspberry Pi |
| darwin/amd64 | Full | Intel Mac |
| darwin/arm64 | Full | Apple Silicon |
| windows/amd64 | Experimental | Edge runtime Windows support limited |

---

## Security Considerations

1. **Binary Verification:**
   - Always verify SHA256 checksum after download
   - Pin to specific edge-runtime version
   - Download only from official GitHub releases

2. **Process Isolation:**
   - Edge runtime runs as separate process
   - Functions run in V8 isolates (sandboxed)
   - No direct filesystem access from functions

3. **Secret Management:**
   - Secrets encrypted at rest (using JWT secret as key)
   - Secrets only accessible within function execution
   - Never logged or exposed via API

4. **JWT Validation:**
   - Validate JWT before proxying (when enabled)
   - Pass validated claims to function
   - Rate limiting on function invocations (future)

5. **Network Security:**
   - Edge runtime binds to localhost only
   - All traffic proxied through sblite
   - HTTPS termination at sblite level

---

## Migration to Supabase

Functions written for sblite should work on Supabase without modification:

1. **Same Runtime:** Both use Supabase Edge Runtime (Deno)
2. **Same APIs:** Deno std library, Supabase client
3. **Same Structure:** `functions/{name}/index.ts` convention
4. **Same Env Vars:** `SUPABASE_URL`, `SUPABASE_ANON_KEY`, etc.

Migration steps:
```bash
# Link to Supabase project
supabase link --project-ref your-project

# Deploy functions
supabase functions deploy hello-world

# Set secrets
supabase secrets set MY_API_KEY=sk-123456
```

---

## Open Questions

1. **Windows Support:** Should we officially support Windows or document it as unsupported?
   - **Recommendation:** Document as experimental, focus on Linux/macOS

2. **Import Maps:** Should we support custom import maps per function?
   - **Recommendation:** Yes, via `functions/{name}/import_map.json`

3. **Shared Code:** How to handle shared code between functions?
   - **Recommendation:** Support `functions/_shared/` directory (Supabase convention)

4. **Background Tasks:** Should we support Deno's long-running workers?
   - **Recommendation:** Future feature, start with request/response model only

5. **Cron/Scheduled Functions:** Should we support scheduled invocations?
   - **Recommendation:** Future feature, can be added with internal cron job

---

## Effort Estimate Summary

| Phase | Effort |
|-------|--------|
| Phase 1: Core Integration | 1-2 weeks |
| Phase 2: Configuration & Secrets | 1 week |
| Phase 3: Dashboard UI | 2-3 weeks |
| Phase 4: Polish & Testing | 1-2 weeks |
| **Total** | **5-8 weeks** |

---

## Dependencies

### External Binary
- Supabase Edge Runtime (~50MB download per platform)
- Version: v1.67.4 or later

### No New Go Dependencies Required
- Uses standard library `net/http/httputil` for proxying
- Uses existing `crypto/sha256` for checksum verification

---

## References

- [Supabase Edge Functions Docs](https://supabase.com/docs/guides/functions)
- [Supabase Edge Runtime GitHub](https://github.com/supabase/edge-runtime)
- [Edge Runtime Self-Hosted Guide](https://supabase.com/blog/edge-runtime-self-hosted-deno-functions)
- [functions.invoke() API Reference](https://supabase.com/docs/reference/javascript/functions-invoke)
- [Deno Deploy](https://deno.com/deploy)
- [supabase-js Source](https://github.com/supabase/supabase-js)
