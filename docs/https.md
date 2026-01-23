# HTTPS with Let's Encrypt

sblite includes built-in HTTPS support using automatic Let's Encrypt certificate provisioning.

## Quick Start

```bash
./sblite serve --https example.com
```

That's it. sblite will:
1. Obtain a certificate from Let's Encrypt
2. Listen on port 443 for HTTPS traffic
3. Listen on port 80 for ACME challenges
4. Redirect HTTP requests to HTTPS

## Requirements

1. **Public domain** - Must be a real domain name (not localhost or IP address)
2. **DNS configured** - Domain must point to your server's IP
3. **Ports accessible** - Ports 80 and 443 must be reachable from the internet

## Configuration

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--https <domain>` | - | Enable HTTPS for this domain |
| `--http-port <port>` | 80 | HTTP port for ACME challenges |
| `--port <port>` | 443 (with HTTPS) | HTTPS port |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SBLITE_HTTPS_DOMAIN` | Alternative to `--https` flag |
| `SBLITE_HTTP_PORT` | Alternative to `--http-port` flag |

## Certificate Storage

Certificates are stored in a `certs/` directory alongside your database:

```
/var/lib/sblite/
├── data.db
└── certs/
    ├── acme_account+key
    └── example.com
```

- Created automatically with mode 0700 (owner-only access)
- Certificates renewed automatically ~30 days before expiration
- Back up this directory along with your database

## Examples

### Basic HTTPS

```bash
./sblite serve --https example.com --db /var/lib/sblite/data.db
```

### With Edge Functions

```bash
./sblite serve --https example.com --functions --functions-dir ./functions
```

### Custom Ports

For non-standard deployments (not recommended for production):

```bash
./sblite serve --https example.com --port 8443 --http-port 8080
```

Note: Let's Encrypt HTTP-01 challenges require port 80 to be accessible.

## Troubleshooting

### "Let's Encrypt requires a public domain"

You're trying to use localhost or an IP address. HTTPS requires a real domain name.

**For local development:** Use HTTP mode or set up a reverse proxy with a self-signed certificate.

### "Cannot bind to port 80/443"

Either:
- Another service is using the port
- You don't have permission (ports < 1024 require root on Unix)

**Solutions:**
- Stop the conflicting service
- Run with `sudo` (not recommended for production)
- Use a reverse proxy instead

### Certificate not obtained

Ensure:
1. DNS is correctly configured (`dig example.com` should show your server IP)
2. Ports 80 and 443 are accessible (not blocked by firewall)
3. Domain is not rate-limited by Let's Encrypt

### Rate Limits

Let's Encrypt has rate limits:
- 50 certificates per domain per week
- 5 duplicate certificates per week

If rate-limited, wait or use Let's Encrypt staging for testing.

## Alternative: Reverse Proxy

If you can't use built-in HTTPS (need custom certs, multiple apps, etc.), use a reverse proxy:

### Nginx Example

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then run sblite on HTTP only:

```bash
./sblite serve --host 127.0.0.1 --port 8080
```

### Caddy Example

```
example.com {
    reverse_proxy localhost:8080
}
```

Caddy handles Let's Encrypt automatically.

## Security Notes

- Certificate private keys are stored in `certs/` directory
- Directory is created with mode 0700 - do not change permissions
- Back up `certs/` along with your database
- HTTPS connections support HTTP/2
