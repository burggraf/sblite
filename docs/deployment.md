# Deployment Guide

This guide covers deploying sblite to a VPS or cloud server for production use.

## Quick Start

```bash
# Build for Linux (from macOS/Windows)
GOOS=linux GOARCH=amd64 go build -o sblite

# Copy to server
scp sblite user@server:/app/
scp -r migrations user@server:/app/

# On server
cd /app
./sblite db push                    # Apply migrations
./sblite serve --host 0.0.0.0       # Start server
```

## What to Deploy

### Required Files

| File/Directory | Description |
|----------------|-------------|
| `sblite` | The compiled binary (build for target OS/architecture) |
| `migrations/` | Your schema migration files |

### Optional Files

| File/Directory | When Needed | Description |
|----------------|-------------|-------------|
| `data.db` | Migrating existing data | Your SQLite database file |
| `functions/` | Using edge functions | TypeScript/JavaScript function files |
| `public/` or custom | Using static hosting | Frontend build files (HTML, CSS, JS) |
| `storage/` | Using local storage backend | Uploaded files (skip if using S3) |

### Files NOT to Deploy

| File | Reason |
|------|--------|
| `*.db-wal`, `*.db-shm` | SQLite temporary files (auto-created) |
| `certs/` | Let's Encrypt certificates (auto-generated) |
| `edge-runtime/` | Edge runtime binary (auto-downloaded) |
| `.env` files | Use environment variables on server instead |

## Building the Binary

### Cross-Compilation

Build for your target platform:

```bash
# Linux AMD64 (most VPS)
GOOS=linux GOARCH=amd64 go build -o sblite

# Linux ARM64 (AWS Graviton, Oracle Ampere)
GOOS=linux GOARCH=arm64 go build -o sblite

# Verify the binary
file sblite
# Output: sblite: ELF 64-bit LSB executable, x86-64...
```

### Optimized Production Build

```bash
# Smaller binary with stripped debug info
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o sblite
```

## Server Setup

### Directory Structure

Recommended production layout:

```
/opt/sblite/
├── sblite              # Binary
├── data.db             # Database (auto-created)
├── migrations/         # Schema migrations
├── functions/          # Edge functions (optional)
├── public/             # Static files (optional)
└── storage/            # Local file storage (optional)
```

### Environment Variables

Create `/etc/sblite/env` or set in your systemd service:

```bash
# Required for production
SBLITE_JWT_SECRET=your-secure-32-character-minimum-secret

# Server binding
SBLITE_HOST=0.0.0.0
SBLITE_PORT=8080

# Database location
SBLITE_DB_PATH=/opt/sblite/data.db

# Email (choose one mode)
SBLITE_MAIL_MODE=smtp
SBLITE_SMTP_HOST=smtp.example.com
SBLITE_SMTP_PORT=587
SBLITE_SMTP_USER=apikey
SBLITE_SMTP_PASS=your-smtp-password
SBLITE_MAIL_FROM=noreply@example.com
SBLITE_SITE_URL=https://example.com

# HTTPS (if using built-in Let's Encrypt)
SBLITE_HTTPS_DOMAIN=example.com

# Static file hosting
SBLITE_STATIC_DIR=/opt/sblite/public

# PostgreSQL wire protocol password (if enabled)
SBLITE_PG_PASSWORD=your-pg-password
```

Generate a secure JWT secret:

```bash
openssl rand -base64 32
```

## Migrations

**Important:** User migrations are NOT automatically applied on startup. You must run them manually.

### Fresh Deployment

```bash
cd /opt/sblite
./sblite db push --migrations-dir ./migrations
```

### Checking Migration Status

```bash
./sblite migration list
```

Output:
```
Applied migrations:
  ✓ 20260117143022_create_users
  ✓ 20260117150000_create_products

Pending migrations:
  • 20260118120000_add_orders
```

### Creating New Migrations

```bash
./sblite migration new add_orders
# Creates: migrations/20260125120000_add_orders.sql
```

## Running as a Service

### systemd (Recommended)

Create `/etc/systemd/system/sblite.service`:

```ini
[Unit]
Description=Supabase Lite Server
After=network.target

[Service]
Type=simple
User=sblite
Group=sblite
WorkingDirectory=/opt/sblite
EnvironmentFile=/etc/sblite/env
ExecStart=/opt/sblite/sblite serve
Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/sblite

[Install]
WantedBy=multi-user.target
```

With edge functions and HTTPS:

```ini
ExecStart=/opt/sblite/sblite serve --functions --https example.com
```

Enable and start:

```bash
# Create service user
sudo useradd -r -s /bin/false sblite
sudo chown -R sblite:sblite /opt/sblite

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable sblite
sudo systemctl start sblite

# Check status
sudo systemctl status sblite
sudo journalctl -u sblite -f
```

### Docker

Create `Dockerfile`:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o sblite

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/sblite .
COPY migrations ./migrations
COPY functions ./functions
COPY public ./public

EXPOSE 8080
VOLUME ["/app/data"]

CMD ["./sblite", "serve", "--host", "0.0.0.0", "--db", "/app/data/data.db"]
```

Run with Docker:

```bash
docker build -t sblite .
docker run -d \
  -p 8080:8080 \
  -v sblite-data:/app/data \
  -e SBLITE_JWT_SECRET=your-secret \
  --name sblite \
  sblite
```

Docker Compose (`docker-compose.yml`):

```yaml
version: '3.8'
services:
  sblite:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - sblite-data:/app/data
      - ./migrations:/app/migrations
      - ./functions:/app/functions
      - ./public:/app/public
    environment:
      - SBLITE_JWT_SECRET=${SBLITE_JWT_SECRET}
      - SBLITE_MAIL_MODE=smtp
      - SBLITE_SMTP_HOST=${SMTP_HOST}
      - SBLITE_SMTP_USER=${SMTP_USER}
      - SBLITE_SMTP_PASS=${SMTP_PASS}
    restart: unless-stopped

volumes:
  sblite-data:
```

## HTTPS Options

### Option 1: Built-in Let's Encrypt (Simplest)

sblite can automatically obtain and renew certificates:

```bash
./sblite serve --https example.com
```

Requirements:
- Ports 80 and 443 accessible from the internet
- DNS A record pointing to your server
- No reverse proxy in front

See [HTTPS Documentation](https.md) for details.

### Option 2: Reverse Proxy with nginx

If you need more control or multiple services:

```nginx
# /etc/nginx/sites-available/sblite
server {
    listen 80;
    server_name example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    # WebSocket support for realtime
    location /realtime/v1/websocket {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;
    }

    # All other requests
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable the site:

```bash
sudo ln -s /etc/nginx/sites-available/sblite /etc/nginx/sites-enabled/
sudo certbot --nginx -d example.com
sudo systemctl reload nginx
```

### Option 3: Cloudflare Tunnel

For servers without public IP or behind NAT:

```bash
# Install cloudflared
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared
chmod +x cloudflared

# Authenticate and create tunnel
./cloudflared tunnel login
./cloudflared tunnel create sblite
./cloudflared tunnel route dns sblite example.com

# Run tunnel
./cloudflared tunnel run --url http://localhost:8080 sblite
```

## Backups

### Database Backup

The simplest approach - copy the database file:

```bash
# Stop writes (optional but safer)
sqlite3 /opt/sblite/data.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Copy database
cp /opt/sblite/data.db /backup/sblite-$(date +%Y%m%d).db
```

Automated daily backup script (`/opt/sblite/backup.sh`):

```bash
#!/bin/bash
BACKUP_DIR=/backup/sblite
DB_PATH=/opt/sblite/data.db
KEEP_DAYS=7

mkdir -p $BACKUP_DIR
sqlite3 $DB_PATH "PRAGMA wal_checkpoint(TRUNCATE);"
cp $DB_PATH $BACKUP_DIR/data-$(date +%Y%m%d-%H%M%S).db

# Clean old backups
find $BACKUP_DIR -name "data-*.db" -mtime +$KEEP_DAYS -delete
```

Add to crontab:

```bash
# Run daily at 3am
0 3 * * * /opt/sblite/backup.sh
```

### Full Backup (with storage)

```bash
tar -czvf sblite-backup-$(date +%Y%m%d).tar.gz \
  /opt/sblite/data.db \
  /opt/sblite/storage/ \
  /opt/sblite/migrations/
```

## Monitoring

### Health Check

sblite provides a health endpoint:

```bash
curl http://localhost:8080/health
# Returns: {"status":"ok"}
```

### Uptime Monitoring

Add to your monitoring service (UptimeRobot, Healthchecks.io, etc.):
- URL: `https://example.com/health`
- Expected response: `{"status":"ok"}`

### Log Monitoring

View logs in real-time:

```bash
# systemd
sudo journalctl -u sblite -f

# File logging
tail -f /var/log/sblite.log

# JSON logging for log aggregators
./sblite serve --log-format=json | your-log-shipper
```

## Security Checklist

- [ ] Set a strong `SBLITE_JWT_SECRET` (32+ characters)
- [ ] Set a strong dashboard password
- [ ] Use HTTPS in production
- [ ] Configure firewall (only expose ports 80, 443)
- [ ] Run as non-root user
- [ ] Enable RLS on all tables with sensitive data
- [ ] Use SMTP mode for production emails (not log/catch)
- [ ] Set `SBLITE_SITE_URL` to your actual domain
- [ ] Regular database backups
- [ ] Monitor health endpoint

### Firewall Setup (ufw)

```bash
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP (for ACME challenges)
sudo ufw allow 443/tcp   # HTTPS
sudo ufw enable
```

## Troubleshooting

### Server won't start

```bash
# Check if port is in use
sudo lsof -i :8080

# Check permissions
ls -la /opt/sblite/

# Run manually to see errors
./sblite serve --log-level=debug
```

### HTTPS certificate errors

```bash
# Check DNS
dig example.com

# Check port accessibility
curl http://example.com/.well-known/acme-challenge/test

# Check certificate status
openssl s_client -connect example.com:443 -servername example.com
```

### Database locked errors

```bash
# Check for stale lock files
ls -la /opt/sblite/*.db*

# Remove WAL files if server is stopped
rm /opt/sblite/data.db-wal /opt/sblite/data.db-shm
```

### Edge functions not working

```bash
# Check if runtime was downloaded
ls -la /opt/sblite/edge-runtime/

# Check function logs
./sblite serve --functions --log-level=debug

# Verify function directory
ls -la /opt/sblite/functions/
```

## Scaling Considerations

sblite is designed for small to medium workloads. Consider migrating to full Supabase when you need:

- Multiple server instances (SQLite doesn't support distributed writes)
- More than ~1000 concurrent users
- Heavy write workloads (>100 writes/second sustained)
- Geographic distribution

Export your schema for migration:

```bash
./sblite migrate export -o schema.sql
```

See the [Supabase migration guide](https://supabase.com/docs/guides/platform/migrating-and-upgrading-projects) for importing into Supabase.
