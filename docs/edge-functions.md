# Edge Functions

sblite supports Supabase-compatible Edge Functions, allowing you to run serverless TypeScript/JavaScript code at the edge. Functions are executed using the [Supabase Edge Runtime](https://github.com/supabase/edge-runtime), providing full compatibility with Supabase's `functions.invoke()` API.

## Overview

Edge Functions in sblite provide:

- **Supabase Compatibility**: Works with `@supabase/supabase-js` client library
- **TypeScript/JavaScript**: Write functions in TypeScript or JavaScript
- **Deno Runtime**: Functions run in Deno-based V8 isolates
- **Automatic Binary Management**: Edge runtime binary is auto-downloaded
- **Secrets Management**: Encrypted storage for API keys and credentials
- **Per-Function Configuration**: JWT verification, memory limits, timeouts

## Quick Start

### 1. Create a Function

```bash
# Create a new function from template
sblite functions new hello-world

# Or with a specific template
sblite functions new my-api --template supabase
sblite functions new webhook --template cors
```

This creates `./functions/hello-world/index.ts`:

```typescript
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

serve(async (req: Request) => {
  const { name } = await req.json()
  const data = {
    message: `Hello ${name || 'World'}!`,
  }
  return new Response(JSON.stringify(data), {
    headers: { "Content-Type": "application/json" },
  })
})
```

### 2. Start the Server with Functions Enabled

```bash
sblite serve --functions
```

### 3. Invoke Your Function

Using curl:
```bash
curl -X POST http://localhost:8080/functions/v1/hello-world \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ANON_KEY" \
  -d '{"name": "World"}'
```

Using the Supabase client:
```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'YOUR_ANON_KEY')

const { data, error } = await supabase.functions.invoke('hello-world', {
  body: { name: 'World' }
})

console.log(data) // { message: 'Hello World!' }
```

## CLI Commands

### Function Management

```bash
# List all functions
sblite functions list

# Create a new function
sblite functions new <name> [--template default|supabase|cors]

# Delete a function
sblite functions delete <name> [--force]

# Download edge runtime binary manually
sblite functions download
```

### Secrets Management

Secrets are encrypted and stored in the database. They are injected as environment variables when the edge runtime starts.

```bash
# Set a secret (prompts for value securely)
sblite functions secrets set API_KEY

# Set a secret with value directly
sblite functions secrets set API_KEY --value "sk-..."

# List all secrets (values are never shown)
sblite functions secrets list

# Delete a secret
sblite functions secrets delete API_KEY [--force]
```

### Per-Function Configuration

```bash
# Show function configuration
sblite functions config show <function-name>

# Enable/disable JWT verification
sblite functions config set-jwt <function-name> enabled
sblite functions config set-jwt <function-name> disabled
```

### Local Development

```bash
# Start edge runtime standalone (for development)
sblite functions serve [--port 8081]

# Start full server with functions
sblite serve --functions [--functions-dir ./functions] [--functions-port 8081]
```

## Configuration

### Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--functions` | `false` | Enable edge functions support |
| `--functions-dir` | `./functions` | Path to functions directory |
| `--functions-port` | `8081` | Internal port for edge runtime |

### Environment Variables

The following environment variables are automatically injected into functions:

| Variable | Description |
|----------|-------------|
| `SUPABASE_URL` | The sblite server URL (e.g., `http://localhost:8080`) |
| `SUPABASE_ANON_KEY` | The anonymous API key |
| `SUPABASE_SERVICE_ROLE_KEY` | The service role API key |
| `SUPABASE_DB_URL` | Database URL (for reference) |

Plus any secrets you've configured with `sblite functions secrets set`.

## Function Templates

### Default Template

Basic function with JSON request/response:

```typescript
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

serve(async (req: Request) => {
  const { name } = await req.json()
  return new Response(JSON.stringify({ message: `Hello ${name}!` }), {
    headers: { "Content-Type": "application/json" },
  })
})
```

### Supabase Template

Function with Supabase client for database access:

```typescript
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from "https://esm.sh/@supabase/supabase-js@2"

serve(async (req: Request) => {
  const supabase = createClient(
    Deno.env.get("SUPABASE_URL")!,
    Deno.env.get("SUPABASE_ANON_KEY")!
  )

  // Your logic here
  const { data, error } = await supabase.from("users").select("*")

  return new Response(JSON.stringify({ data, error }), {
    headers: { "Content-Type": "application/json" },
  })
})
```

### CORS Template

Function with CORS headers for browser requests:

```typescript
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "authorization, x-client-info, apikey, content-type",
}

serve(async (req: Request) => {
  if (req.method === "OPTIONS") {
    return new Response("ok", { headers: corsHeaders })
  }

  const { name } = await req.json()
  return new Response(JSON.stringify({ message: `Hello ${name}!` }), {
    headers: { ...corsHeaders, "Content-Type": "application/json" },
  })
})
```

## JWT Verification

By default, all functions require a valid JWT token in the Authorization header. You can disable this for specific functions (useful for webhooks):

```bash
# Disable JWT verification for a webhook function
sblite functions config set-jwt my-webhook disabled
```

When JWT verification is disabled, the function can be invoked without an Authorization header.

## Dashboard API

The dashboard provides API endpoints for managing functions:

### Functions

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_/api/functions` | GET | List all functions |
| `/_/api/functions/status` | GET | Get edge runtime status |
| `/_/api/functions/{name}` | GET | Get function details |
| `/_/api/functions/{name}` | POST | Create function |
| `/_/api/functions/{name}` | DELETE | Delete function |
| `/_/api/functions/{name}/config` | GET | Get function config |
| `/_/api/functions/{name}/config` | PATCH | Update function config |

### Secrets

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_/api/secrets` | GET | List all secrets (names only) |
| `/_/api/secrets` | POST | Set a secret |
| `/_/api/secrets/{name}` | DELETE | Delete a secret |

## Dashboard UI

The web dashboard includes a visual interface for managing edge functions. Access it at `http://localhost:8080/_` and navigate to the "Edge Functions" section.

### Features

- **Functions List**: View all deployed functions with their status
- **Runtime Status**: See if the edge runtime is running
- **Function Details**: View endpoint URLs and configuration for each function
- **JWT Configuration**: Toggle JWT verification per function
- **Test Console**: Invoke functions directly from the browser with custom headers and body
- **Secrets Management**: Add and remove secrets (values are never displayed)

### Function Test Console

The test console allows you to invoke functions directly from the dashboard:

1. Select a function from the list
2. Choose HTTP method (GET, POST, PUT, PATCH, DELETE)
3. Add custom headers if needed
4. Enter a request body (JSON format for POST/PUT/PATCH)
5. Click "Invoke" to send the request
6. View the response status, timing, and body

API keys are automatically injected into requests when available.

### Secrets Panel

The secrets panel allows you to manage environment variables for your functions:

- **Add Secret**: Click "+ Add Secret" and enter a name (uppercase with underscores) and value
- **View Secrets**: Only secret names are shown; values are never exposed
- **Delete Secret**: Remove secrets you no longer need

Note: Secrets require a server restart to take effect in the edge runtime.

## Database Schema

Edge functions use three database tables for configuration:

### `_functions_config`

Global configuration key-value store.

```sql
CREATE TABLE _functions_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
```

### `_functions_secrets`

Encrypted secrets storage (AES-GCM encryption).

```sql
CREATE TABLE _functions_secrets (
    name TEXT PRIMARY KEY,
    value TEXT NOT NULL,  -- Encrypted
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
```

### `_functions_metadata`

Per-function configuration.

```sql
CREATE TABLE _functions_metadata (
    name TEXT PRIMARY KEY,
    verify_jwt INTEGER DEFAULT 1,
    memory_mb INTEGER,
    timeout_ms INTEGER,
    import_map TEXT,
    env_vars TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
```

## Supported Platforms

The edge runtime binary is available for:

| Platform | Architecture |
|----------|--------------|
| macOS | x86_64, arm64 |
| Linux | x86_64, arm64 |

Windows is not currently supported.

## Troubleshooting

### Edge runtime not starting

1. Check if the binary was downloaded: `ls ~/.sblite/bin/edge-runtime`
2. Try downloading manually: `sblite functions download`
3. Check logs for errors: `sblite serve --functions --log-level debug`

### Function not found (404)

1. Verify the function exists: `sblite functions list`
2. Check that the function has an `index.ts` or `index.js` file
3. Ensure the functions directory is correct: `--functions-dir ./functions`

### Secrets not available in function

Secrets are loaded when the edge runtime starts. After setting secrets:
1. Restart the server or edge runtime
2. Verify secrets are set: `sblite functions secrets list`

### JWT verification errors

1. Ensure you're passing a valid Authorization header
2. Check if JWT verification is enabled: `sblite functions config show <name>`
3. For public functions, disable verification: `sblite functions config set-jwt <name> disabled`

## Differences from Supabase

| Feature | sblite | Supabase |
|---------|--------|----------|
| Runtime | Supabase Edge Runtime | Supabase Edge Runtime |
| Secrets hot-reload | Requires restart | Hot-reload |
| Import maps | Per-function | Global + per-function |
| Streaming responses | Not supported | Supported |
| WebSocket | Not supported | Supported |
| Blob/FormData | Not supported | Supported |

## Migration to Supabase

When migrating to full Supabase:

1. Copy your `functions/` directory to your Supabase project
2. Migrate secrets using `supabase secrets set`
3. Deploy with `supabase functions deploy`

Your function code should work without modifications since sblite uses the same Supabase Edge Runtime.
