# Static File Hosting

sblite can serve static files (HTML, CSS, JavaScript, images) alongside your API, enabling you to host your frontend and backend from a single binary.

## Quick Start

```bash
# Create a public directory with your frontend
mkdir public
echo '<html><body><h1>Hello World</h1></body></html>' > public/index.html

# Start the server (serves from ./public by default)
./sblite serve
```

Your frontend is now available at `http://localhost:8080/` and your API at `http://localhost:8080/rest/v1/`.

## Configuration

| CLI Flag | Environment Variable | Default | Description |
|----------|---------------------|---------|-------------|
| `--static-dir` | `SBLITE_STATIC_DIR` | `./public` | Directory containing static files |

**Priority:** CLI flag > environment variable > default

```bash
# Use CLI flag
./sblite serve --static-dir ./dist

# Use environment variable
export SBLITE_STATIC_DIR=./frontend/build
./sblite serve
```

## How It Works

### Route Priority

API routes always take priority over static files. The routing order is:

1. `/health` - Health check endpoint
2. `/auth/v1/*` - Authentication API
3. `/rest/v1/*` - REST API
4. `/storage/v1/*` - Storage API
5. `/functions/v1/*` - Edge Functions
6. `/admin/v1/*` - Admin API
7. `/_/*` - Dashboard
8. `/realtime/v1/*` - WebSocket connections
9. `/*` - Static files (catch-all)

This means you can safely use any path for your frontend routes without conflicting with the API.

### SPA Fallback

sblite includes built-in support for Single Page Applications (SPAs) like React, Vue, Angular, and Svelte.

**How it works:**
- Requests for files with extensions (`.js`, `.css`, `.png`, etc.) are served directly
- Requests for paths without extensions (`/about`, `/dashboard`, `/users/123`) return `index.html`
- This allows your client-side router to handle navigation

**Example:**
```
GET /                    → serves index.html
GET /about               → serves index.html (SPA route)
GET /users/123           → serves index.html (SPA route)
GET /style.css           → serves style.css
GET /app.js              → serves app.js
GET /images/logo.png     → serves images/logo.png
GET /missing.js          → 404 (file with extension not found)
```

### Silent Skip

If the static directory doesn't exist, sblite silently continues without static file serving. This is useful for:
- API-only deployments where no frontend is needed
- Development environments where frontend runs separately
- Gradual migration from separate frontend hosting

## Use Cases

### Hosting a React App

```bash
# Build your React app
cd my-react-app
npm run build

# Serve with sblite
./sblite serve --static-dir ./my-react-app/build
```

### Hosting a Vue App

```bash
# Build your Vue app
cd my-vue-app
npm run build

# Serve with sblite
./sblite serve --static-dir ./my-vue-app/dist
```

### Hosting a Static Site

```bash
# Create your site structure
mkdir -p public/css public/js public/images
echo '<html>...</html>' > public/index.html
echo 'body { ... }' > public/css/style.css

# Serve
./sblite serve
```

### Production with HTTPS

```bash
# Serve frontend + API with automatic HTTPS
./sblite serve --static-dir ./dist --https example.com
```

## Frontend Configuration

### Connecting to the API

Since your frontend and API share the same origin, you can use relative URLs:

```javascript
import { createClient } from '@supabase/supabase-js'

// Use relative URL (same origin)
const supabase = createClient('', 'your-anon-key')

// Or use the full URL
const supabase = createClient('http://localhost:8080', 'your-anon-key')
```

### Environment Variables in Build

For production builds, configure your frontend's API URL:

```bash
# React (Create React App)
REACT_APP_SUPABASE_URL=https://api.example.com npm run build

# Vue (Vite)
VITE_SUPABASE_URL=https://api.example.com npm run build
```

## Directory Structure Example

A typical project structure:

```
my-project/
├── data.db              # sblite database
├── public/              # Static files (default location)
│   ├── index.html
│   ├── favicon.ico
│   ├── assets/
│   │   ├── app.js
│   │   └── style.css
│   └── images/
│       └── logo.png
├── migrations/          # Database migrations
│   └── 20240101000000_create_users.sql
└── sblite               # sblite binary
```

## MIME Types

sblite automatically detects and sets the correct `Content-Type` header based on file extensions:

| Extension | Content-Type |
|-----------|-------------|
| `.html` | `text/html` |
| `.css` | `text/css` |
| `.js` | `application/javascript` |
| `.json` | `application/json` |
| `.png` | `image/png` |
| `.jpg`, `.jpeg` | `image/jpeg` |
| `.gif` | `image/gif` |
| `.svg` | `image/svg+xml` |
| `.ico` | `image/x-icon` |
| `.woff`, `.woff2` | `font/woff`, `font/woff2` |
| `.ttf` | `font/ttf` |
| `.pdf` | `application/pdf` |

Other files are served with their detected MIME type or `application/octet-stream`.

## Security

### Path Traversal Protection

sblite prevents directory traversal attacks. Requests like `/../etc/passwd` are blocked - only files within the configured static directory can be accessed.

### No Directory Listings

Directory listings are disabled. Requests to directories without an `index.html` return the SPA fallback (root `index.html`) or 404.

## Comparison with Alternatives

| Feature | sblite | nginx + API | Separate hosting |
|---------|--------|-------------|------------------|
| Single binary | Yes | No | No |
| Configuration | Minimal | Complex | Varies |
| SPA support | Built-in | Requires config | Varies |
| HTTPS | Auto (Let's Encrypt) | Manual/Certbot | Varies |
| Deployment | Simple | Multiple services | Multiple services |

## Troubleshooting

### Static files not being served

1. Check if the directory exists: `ls -la ./public`
2. Check server logs for "serving static files" message
3. Verify the path is correct (relative to where sblite runs)

### SPA routes returning 404

Ensure your SPA's `index.html` exists at the root of your static directory:
```bash
ls ./public/index.html
```

### API routes not working

API routes (`/auth/v1/*`, `/rest/v1/*`, etc.) always take priority. If you're getting static content instead of API responses, check:
1. You're using the correct API path prefix
2. The `apikey` header is set correctly

### Wrong Content-Type

Go's `http.ServeFile` detects MIME types automatically. If you're seeing incorrect types:
1. Check the file extension is correct
2. Ensure the file isn't corrupted
3. Try adding the appropriate extension
