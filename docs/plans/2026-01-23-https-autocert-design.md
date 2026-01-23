# HTTPS Support with Let's Encrypt Autocert

**Date:** 2026-01-23
**Status:** Draft

## Overview

Add built-in HTTPS support to sblite using automatic Let's Encrypt certificate provisioning, matching PocketBase's approach. This enables single-command secure deployments without external reverse proxies.

## Requirements

- Automatic certificate provisioning via Let's Encrypt (ACME protocol)
- Zero-configuration certificate renewal
- Clear error messages for invalid configurations (localhost, IPs)
- Works with edge functions
- Certificates stored alongside database for easy backup

## CLI Interface

sblite keeps its existing flag style while adding HTTPS support:

```bash
# HTTP only (current behavior, unchanged)
./sblite serve --port 8080

# HTTPS with automatic Let's Encrypt
./sblite serve --https example.com

# HTTPS with custom ports
./sblite serve --https example.com --port 8443 --http-port 8080

# HTTPS with edge functions
./sblite serve --https example.com --functions
```

### New Flags

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--https` | `SBLITE_HTTPS_DOMAIN` | (none) | Domain for Let's Encrypt HTTPS |
| `--http-port` | `SBLITE_HTTP_PORT` | `80` | HTTP port for ACME challenges (only when --https set) |

### Behavior When `--https` Is Set

1. Validates domain is not localhost/IP address (fails with clear error)
2. Binds to port 80 (or `--http-port`) for ACME HTTP-01 challenges
3. Binds to port 443 (or `--port`) for HTTPS traffic
4. HTTP requests redirect to HTTPS (except `/.well-known/acme-challenge/*`)

## Certificate Storage

Certificates stored alongside the database file:

```
data.db                    # SQLite database
certs/                     # Certificate cache directory (auto-created)
├── acme_account+key       # ACME account credentials
├── example.com            # Cached certificate for domain
└── example.com+rsa        # Private key
```

- Default location: `<db-dir>/certs/`
- Directory created automatically with mode 0700 (owner-only)
- Uses Go's `autocert.DirCache` for standard ACME caching

### Certificate Lifecycle

- Obtained automatically on first HTTPS request
- Renewed automatically ~30 days before expiration
- Failed renewals retried on subsequent requests
- No manual intervention required

## Implementation Architecture

### New File: `internal/server/https.go`

```go
package server

import (
    "crypto/tls"
    "net/http"
    "golang.org/x/crypto/acme/autocert"
)

// HTTPSConfig holds HTTPS configuration
type HTTPSConfig struct {
    Domain   string // Required: domain for Let's Encrypt
    CertDir  string // Certificate cache directory
    HTTPPort string // Port for ACME challenges (default ":80")
}

// NewAutocertManager creates an autocert manager for the domain
func NewAutocertManager(cfg HTTPSConfig) *autocert.Manager {
    return &autocert.Manager{
        Prompt:     autocert.AcceptTOS,
        HostPolicy: autocert.HostWhitelist(cfg.Domain),
        Cache:      autocert.DirCache(cfg.CertDir),
    }
}

// HTTPRedirectHandler redirects HTTP to HTTPS (except ACME challenges)
func HTTPRedirectHandler(domain string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        target := "https://" + domain + r.URL.RequestURI()
        http.Redirect(w, r, target, http.StatusMovedPermanently)
    })
}
```

### Changes to `internal/server/server.go`

Add new method:

```go
// ListenAndServeTLS starts HTTPS server with autocert
func (s *Server) ListenAndServeTLS(httpsAddr, httpAddr string, cfg HTTPSConfig) error {
    manager := NewAutocertManager(cfg)

    // HTTPS server
    s.httpServer = &http.Server{
        Addr:      httpsAddr,
        Handler:   s.router,
        TLSConfig: &tls.Config{GetCertificate: manager.GetCertificate},
    }

    // HTTP server for ACME challenges + redirects
    httpServer := &http.Server{
        Addr:    httpAddr,
        Handler: manager.HTTPHandler(HTTPRedirectHandler(cfg.Domain)),
    }

    // Start both servers...
}
```

### Changes to `cmd/serve.go`

1. Add `--https` and `--http-port` flags
2. Domain validation function
3. Conditional startup logic:

```go
if httpsEnabled {
    // Validate domain
    if err := validateDomain(domain); err != nil {
        return err
    }

    // Update BaseURL for edge functions
    baseURL = "https://" + domain

    // Start dual servers
    return srv.ListenAndServeTLS(httpsAddr, httpAddr, httpsCfg)
} else {
    // Existing HTTP-only path
    return srv.ListenAndServe(addr)
}
```

## Error Handling

### Domain Validation Errors

| Input | Error Message |
|-------|---------------|
| `localhost` | "Let's Encrypt requires a public domain, not localhost. Use a reverse proxy for local HTTPS." |
| `127.0.0.1` | "Let's Encrypt requires a domain name, not an IP address." |
| `192.168.1.1` | "Let's Encrypt requires a public domain, not a private IP address." |
| (empty) | "Domain required for --https flag." |
| `invalid..domain` | "Invalid domain format: invalid..domain" |

### Port Binding Errors

| Error | Message |
|-------|---------|
| Port 80 in use | "Cannot bind to port 80 for ACME challenges. Either free the port or use a reverse proxy for HTTPS." |
| Port 443 in use | "Cannot bind to port 443 for HTTPS. Check if another service is using this port." |
| Permission denied | "Ports 80/443 require root privileges. Run with sudo or use --port/--http-port for higher ports." |

### Runtime Errors

- Certificate fetch failures logged with domain and error details
- Rate limit errors include guidance to wait before retrying
- Renewal failures logged but don't stop the server (existing cert continues working)

## Edge Functions Integration

Edge functions work transparently with HTTPS:

1. **Internal proxy unchanged** - Edge runtime stays HTTP on localhost (port 8081). TLS terminates at the main server.

2. **BaseURL updated** - When HTTPS enabled:
   ```go
   // HTTP mode
   BaseURL: "http://0.0.0.0:8080"

   // HTTPS mode
   BaseURL: "https://example.com"
   ```

3. **SUPABASE_URL in functions** - Edge functions receive the HTTPS URL for API callbacks.

No changes needed to edge runtime, function routing, or JWT verification.

## Graceful Shutdown

When HTTPS is enabled, both servers shut down on SIGINT/SIGTERM:

```go
go func() {
    <-sigCh
    log.Info("shutting down...")

    // Shutdown both servers with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    httpServer.Shutdown(ctx)   // HTTP/ACME server
    httpsServer.Shutdown(ctx)  // HTTPS server
}()
```

## Files to Create/Modify

### Create

| File | Description |
|------|-------------|
| `internal/server/https.go` | HTTPS server, autocert manager, redirect handler |
| `docs/https.md` | Comprehensive HTTPS documentation |

### Modify

| File | Changes |
|------|---------|
| `cmd/serve.go` | Add flags, domain validation, dual-server startup |
| `internal/server/server.go` | Add `ListenAndServeTLS` method, extend shutdown |
| `CLAUDE.md` | Add HTTPS configuration section |

## Dependencies

```go
import "golang.org/x/crypto/acme/autocert"
```

This is part of Go's extended standard library (golang.org/x/crypto). No external dependencies.

## Testing Strategy

1. **Unit tests** - Domain validation, redirect handler
2. **Integration tests** - Use Pebble (Let's Encrypt test server) for ACME flow testing
3. **Manual testing** - Test against Let's Encrypt staging environment before production

## Out of Scope

These features are explicitly not included (users should use a reverse proxy):

- Custom certificate files (cert.pem/key.pem)
- Self-signed certificates for local development
- Multiple domains / SAN certificates
- Wildcard certificates
- ACME DNS-01 challenges

## Example Usage

### Basic HTTPS Deployment

```bash
# Prerequisites:
# 1. Domain DNS points to this server
# 2. Ports 80 and 443 are accessible

./sblite serve --https example.com --db /var/lib/sblite/data.db
```

Output:
```
INFO starting server addr=0.0.0.0:443 https=true domain=example.com
INFO ACME challenge server addr=0.0.0.0:80
INFO certificate obtained domain=example.com
INFO auth_api=https://example.com/auth/v1
INFO rest_api=https://example.com/rest/v1
```

### HTTPS with Edge Functions

```bash
./sblite serve --https example.com --functions --functions-dir ./functions
```

Output:
```
INFO starting server addr=0.0.0.0:443 https=true domain=example.com
INFO edge functions enabled functions_api=https://example.com/functions/v1
```
