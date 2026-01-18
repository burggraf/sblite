# sblite File Storage Design

**Date:** 2026-01-18
**Status:** Proposed
**Priority:** Medium (Major Feature)

## Overview

This document outlines the design for implementing Supabase-compatible file storage in sblite with support for both local filesystem and S3-compatible storage backends (similar to Pocketbase's approach).

## Research Summary

### Supabase Storage API

**Sources:**
- [Supabase Storage Docs](https://supabase.com/docs/guides/storage)
- [Supabase Storage JS Client](https://supabase.com/docs/reference/javascript/storage-from-upload)
- [Supabase Storage Schema](https://supabase.com/docs/guides/storage/schema/design)
- [Supabase Storage Access Control](https://supabase.com/docs/guides/storage/security/access-control)
- [Supabase Storage Image Transformations](https://supabase.com/docs/guides/storage/serving/image-transformations)
- [DeepWiki Supabase Storage Core API](https://deepwiki.com/supabase/storage/2-core-api)

**Key Findings:**

1. **REST API Structure:**
   - Bucket Operations: `/bucket`, `/bucket/:id`
   - Object Operations: `/object/:bucketName/*`
   - Signed URLs: `/object/sign/:bucketName/*`
   - Image Render: `/render/image/public/:bucketName/*`

2. **Database Schema:**
   - Metadata stored in `storage` schema in PostgreSQL
   - Main tables: `storage.buckets`, `storage.objects`
   - RLS policies applied to `storage.objects` table

3. **Access Control:**
   - RLS on `storage.objects` table for fine-grained permissions
   - Buckets can be public (bypass read RLS) or private
   - Operations: SELECT (download), INSERT (upload), UPDATE, DELETE

4. **Image Transformations:**
   - On-the-fly resizing via `/render/image/` endpoint
   - Parameters: width, height, quality (20-100), resize mode (cover/contain/fill)
   - Uses imgproxy under the hood (self-hosted)

### Pocketbase Storage Approach

**Sources:**
- [Pocketbase Filesystem Docs](https://pocketbase.io/docs/go-filesystem/)
- [Pocketbase filesystem.go source](https://github.com/pocketbase/pocketbase/blob/master/tools/filesystem/filesystem.go)
- [DeepWiki Pocketbase Storage Systems](https://deepwiki.com/pocketbase/pocketbase/6-storage-systems)

**Key Findings:**

1. **Dual Backend Support:**
   - Local filesystem (default): stores in `pb_data/storage/`
   - S3-compatible: any S3 API (AWS, MinIO, Backblaze, etc.)
   - Configuration via Dashboard settings

2. **System Interface:**
   ```go
   type System struct {
       ctx    context.Context
       bucket *blob.Bucket
   }
   ```
   - `NewLocal(dirPath string) (*System, error)`
   - `NewS3(bucket, region, endpoint, accessKey, secretKey string, forcePathStyle bool) (*System, error)`

3. **Operations:**
   - `Exists(fileKey)`, `Attributes(fileKey)`, `GetReader(fileKey)`
   - `Upload(content, fileKey)`, `UploadFile(file, fileKey)`, `UploadMultipart(fh, fileKey)`
   - `Delete(fileKey)`, `DeletePrefix(prefix)`, `List(prefix)`
   - `Copy(srcKey, dstKey)`, `CreateThumb(origKey, thumbKey, thumbSize)`
   - `Serve(res, req, fileKey, name)` - serves with proper HTTP headers

4. **File Key Structure:**
   - `collectionId/recordId/filename`
   - Thumbnails: `collectionId/recordId/thumbs_filename/size_filename`

### Go Libraries for S3

**Sources:**
- [minio-go](https://github.com/minio/minio-go) - Lightweight, S3-focused SDK
- [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) - Full AWS SDK (v1 deprecated July 2025)
- [disintegration/imaging](https://pkg.go.dev/github.com/disintegration/imaging) - Pure Go image processing

**Recommendation:** Use `minio-go` for S3 operations:
- Lighter weight than AWS SDK
- Specifically designed for S3-compatible storage
- Simple API, good documentation
- Works with any S3-compatible endpoint

---

## Design Goals

1. **Supabase Client Compatibility** - Work with `@supabase/supabase-js` storage client
2. **Dual Backend Support** - Local filesystem (default) and S3-compatible storage
3. **RLS Integration** - Use existing sblite RLS system for access control
4. **Image Transformations** - On-the-fly resize/transform using pure Go
5. **No CGO** - Maintain pure Go, single binary distribution
6. **Migration Path** - Export to Supabase Storage when migrating

---

## Database Schema

### storage_buckets Table

```sql
CREATE TABLE IF NOT EXISTS storage_buckets (
    id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    name          TEXT UNIQUE NOT NULL,
    owner         TEXT REFERENCES auth_users(id),
    public        INTEGER DEFAULT 0,
    file_size_limit   INTEGER,          -- Max file size in bytes (NULL = no limit)
    allowed_mime_types TEXT,            -- JSON array of allowed MIME types (NULL = all)
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_storage_buckets_name ON storage_buckets(name);
```

### storage_objects Table

```sql
CREATE TABLE IF NOT EXISTS storage_objects (
    id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    bucket_id     TEXT NOT NULL REFERENCES storage_buckets(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,        -- Full path within bucket (e.g., "folder/file.png")
    owner         TEXT REFERENCES auth_users(id),
    metadata      TEXT DEFAULT '{}' CHECK (json_valid(metadata)),
    path_tokens   TEXT,                 -- JSON array of path segments for RLS
    version       TEXT,                 -- For versioning support (future)
    size          INTEGER NOT NULL,     -- File size in bytes
    mime_type     TEXT,                 -- Content-Type
    etag          TEXT,                 -- MD5 hash for caching
    last_accessed_at TEXT,
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now')),
    UNIQUE(bucket_id, name)
);

CREATE INDEX IF NOT EXISTS idx_storage_objects_bucket ON storage_objects(bucket_id);
CREATE INDEX IF NOT EXISTS idx_storage_objects_name ON storage_objects(name);
CREATE INDEX IF NOT EXISTS idx_storage_objects_owner ON storage_objects(owner);
CREATE INDEX IF NOT EXISTS idx_storage_objects_path ON storage_objects(bucket_id, name);
```

### storage_config Table (Settings)

```sql
CREATE TABLE IF NOT EXISTS storage_config (
    key           TEXT PRIMARY KEY,
    value         TEXT NOT NULL,
    updated_at    TEXT DEFAULT (datetime('now'))
);

-- Keys:
-- 'backend' = 'local' | 's3'
-- 's3_endpoint' = S3 endpoint URL
-- 's3_region' = S3 region
-- 's3_bucket' = S3 bucket name
-- 's3_access_key' = Access key (encrypted)
-- 's3_secret_key' = Secret key (encrypted)
-- 's3_force_path_style' = '0' | '1'
-- 'local_path' = Local storage directory path
```

---

## REST API Endpoints

All endpoints prefixed with `/storage/v1/`

### Bucket Operations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/bucket` | List all buckets | JWT/Service |
| GET | `/bucket/{id}` | Get bucket details | JWT/Service |
| POST | `/bucket` | Create bucket | Service only |
| PUT | `/bucket/{id}` | Update bucket | Service only |
| DELETE | `/bucket/{id}` | Delete bucket (must be empty) | Service only |
| POST | `/bucket/{id}/empty` | Empty bucket | Service only |

### Object Operations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/object/{bucket}/{*path}` | Download file | JWT/Signed URL |
| HEAD | `/object/{bucket}/{*path}` | Get file metadata | JWT/Signed URL |
| POST | `/object/{bucket}/{*path}` | Upload file | JWT (with RLS) |
| PUT | `/object/{bucket}/{*path}` | Update/replace file | JWT (with RLS) |
| DELETE | `/object/{bucket}/{*path}` | Delete file | JWT (with RLS) |
| POST | `/object/copy` | Copy file | JWT (with RLS) |
| POST | `/object/move` | Move file | JWT (with RLS) |
| POST | `/object/list/{bucket}` | List objects in bucket | JWT (with RLS) |

### Signed URL Operations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/object/sign/{bucket}/{*path}` | Create signed download URL | JWT |
| POST | `/object/upload/sign/{bucket}/{*path}` | Create signed upload URL | JWT |
| PUT | `/object/upload/{bucket}/{*path}` | Upload to signed URL | Signed URL |

### Public URL Operations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/object/public/{bucket}/{*path}` | Get file from public bucket | None |

### Image Transformation

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/render/image/public/{bucket}/{*path}` | Transformed image (public) | None |
| GET | `/render/image/authenticated/{bucket}/{*path}` | Transformed image (auth) | JWT/Signed |

**Query Parameters:**
- `width` - Target width (1-2500)
- `height` - Target height (1-2500)
- `quality` - JPEG/WebP quality (20-100, default 80)
- `resize` - Mode: `cover` (default), `contain`, `fill`
- `format` - Output format: `origin`, `webp`, `jpeg`, `png`

---

## Package Structure

```
internal/
├── storage/
│   ├── storage.go          # Main service, orchestrates operations
│   ├── bucket.go           # Bucket CRUD operations
│   ├── object.go           # Object operations (upload, download, delete)
│   ├── signed_url.go       # Signed URL generation and validation
│   ├── transform.go        # Image transformation logic
│   ├── handler.go          # HTTP handlers for /storage/v1/*
│   ├── middleware.go       # Storage-specific middleware
│   │
│   ├── backend/
│   │   ├── backend.go      # Backend interface definition
│   │   ├── local.go        # Local filesystem backend
│   │   └── s3.go           # S3-compatible backend
│   │
│   └── storage_test.go     # Tests
```

---

## Backend Interface

```go
// internal/storage/backend/backend.go
package backend

import (
    "context"
    "io"
)

// FileInfo represents metadata about a stored file.
type FileInfo struct {
    Key          string
    Size         int64
    ContentType  string
    ETag         string
    ModTime      time.Time
}

// Backend defines the interface for storage backends.
type Backend interface {
    // Exists checks if a file exists at the given key.
    Exists(ctx context.Context, key string) (bool, error)

    // Attributes returns metadata for a file.
    Attributes(ctx context.Context, key string) (*FileInfo, error)

    // Reader returns a reader for the file content.
    Reader(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)

    // Write stores content at the given key.
    Write(ctx context.Context, key string, content io.Reader, size int64, contentType string) (*FileInfo, error)

    // Delete removes a file.
    Delete(ctx context.Context, key string) error

    // DeletePrefix removes all files with the given prefix.
    DeletePrefix(ctx context.Context, prefix string) error

    // List returns all files with the given prefix.
    List(ctx context.Context, prefix string, limit int, offset string) ([]FileInfo, string, error)

    // Copy duplicates a file to a new key.
    Copy(ctx context.Context, srcKey, dstKey string) error

    // Close releases any resources.
    Close() error
}
```

### Local Backend Implementation

```go
// internal/storage/backend/local.go
package backend

type LocalBackend struct {
    basePath string
}

func NewLocal(basePath string) (*LocalBackend, error) {
    // Create base directory if not exists
    if err := os.MkdirAll(basePath, 0755); err != nil {
        return nil, err
    }
    return &LocalBackend{basePath: basePath}, nil
}

// File storage path: basePath/bucket_id/object_name
// This ensures flat structure within each bucket for easy management
```

### S3 Backend Implementation

```go
// internal/storage/backend/s3.go
package backend

import (
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
    Endpoint        string
    Region          string
    Bucket          string
    AccessKey       string
    SecretKey       string
    ForcePathStyle  bool
    UseSSL          bool
}

type S3Backend struct {
    client *minio.Client
    bucket string
}

func NewS3(cfg S3Config) (*S3Backend, error) {
    client, err := minio.New(cfg.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
        Secure: cfg.UseSSL,
        Region: cfg.Region,
    })
    if err != nil {
        return nil, err
    }

    return &S3Backend{
        client: client,
        bucket: cfg.Bucket,
    }, nil
}

// File storage key: bucket_id/object_name
// Same structure as local for consistency
```

---

## Image Transformation

Using `disintegration/imaging` (pure Go, no CGO):

```go
// internal/storage/transform.go
package storage

import (
    "image"
    "github.com/disintegration/imaging"
)

type TransformOptions struct {
    Width   int    // Target width (0 = auto)
    Height  int    // Target height (0 = auto)
    Quality int    // JPEG/WebP quality (20-100)
    Resize  string // "cover", "contain", "fill"
    Format  string // "origin", "webp", "jpeg", "png"
}

func (s *Service) TransformImage(src io.Reader, opts TransformOptions) (io.Reader, string, error) {
    img, format, err := image.Decode(src)
    if err != nil {
        return nil, "", err
    }

    // Apply resize based on mode
    var resized image.Image
    switch opts.Resize {
    case "cover":
        resized = imaging.Fill(img, opts.Width, opts.Height, imaging.Center, imaging.Lanczos)
    case "contain":
        resized = imaging.Fit(img, opts.Width, opts.Height, imaging.Lanczos)
    case "fill":
        resized = imaging.Resize(img, opts.Width, opts.Height, imaging.Lanczos)
    default:
        resized = imaging.Fit(img, opts.Width, opts.Height, imaging.Lanczos)
    }

    // Encode to output format
    // ...
}
```

**Transformation Caching:**
- Cache transformed images locally (even when using S3 backend)
- Cache key: `_cache/{bucket}/{hash(path+options)}.{format}`
- LRU eviction based on configurable max cache size
- Cache headers: `Cache-Control: public, max-age=31536000, immutable`

---

## Signed URL Implementation

```go
// internal/storage/signed_url.go
package storage

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "time"
)

type SignedURLClaims struct {
    BucketID  string `json:"bid"`
    Path      string `json:"path"`
    ExpiresAt int64  `json:"exp"`
    Transform *TransformOptions `json:"tr,omitempty"` // Optional transforms baked in
}

func (s *Service) CreateSignedURL(bucketID, path string, expiresIn time.Duration, transform *TransformOptions) (string, error) {
    claims := SignedURLClaims{
        BucketID:  bucketID,
        Path:      path,
        ExpiresAt: time.Now().Add(expiresIn).Unix(),
        Transform: transform,
    }

    // Sign with HMAC-SHA256 using JWT secret
    payload, _ := json.Marshal(claims)
    signature := s.signPayload(payload)

    token := base64.URLEncoding.EncodeToString(payload) + "." + signature

    return fmt.Sprintf("/storage/v1/object/sign/%s/%s?token=%s", bucketID, path, token), nil
}

func (s *Service) ValidateSignedURL(token string) (*SignedURLClaims, error) {
    // Decode and verify signature
    // Check expiration
    // Return claims or error
}
```

---

## RLS Integration

Storage uses the existing sblite RLS system. Policies are created on `storage_objects` table:

```sql
-- Example: Users can only access their own files
CREATE POLICY "Users own files" ON storage_objects
FOR ALL
USING (owner = auth.uid());

-- Example: Public bucket access
CREATE POLICY "Public bucket read" ON storage_objects
FOR SELECT
USING (
    bucket_id IN (SELECT id FROM storage_buckets WHERE public = 1)
);

-- Example: Folder-based access (using path_tokens JSON)
CREATE POLICY "Folder access" ON storage_objects
FOR SELECT
USING (
    json_extract(path_tokens, '$[0]') = auth.uid()
);
```

**RLS Context Variables:**
- `auth.uid()` - Current user ID from JWT
- `auth.role()` - User role from JWT
- `auth.jwt()` - Full JWT claims as JSON

---

## CLI Commands

### Storage Configuration

```bash
# View current storage config
./sblite storage config

# Set local storage path
./sblite storage config --backend local --path /var/data/storage

# Configure S3 storage
./sblite storage config --backend s3 \
    --s3-endpoint s3.amazonaws.com \
    --s3-region us-east-1 \
    --s3-bucket my-bucket \
    --s3-access-key AKIAIOSFODNN7EXAMPLE \
    --s3-secret-key wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Storage Management

```bash
# List buckets
./sblite storage buckets list

# Create bucket
./sblite storage buckets create avatars --public

# Delete bucket
./sblite storage buckets delete avatars

# List objects in bucket
./sblite storage objects list avatars

# Upload file
./sblite storage objects upload avatars path/to/local/file.png remote/path.png

# Download file
./sblite storage objects download avatars remote/path.png local/file.png

# Delete file
./sblite storage objects delete avatars remote/path.png
```

---

## Dashboard Integration

Add storage management to the web dashboard:

### Storage Tab
- **Buckets List:** Create, edit, delete buckets
- **File Browser:** Navigate folders, preview images, upload/download
- **Settings:** Configure backend (local/S3), view storage usage

### Dashboard Endpoints

```
GET    /_/api/storage/buckets              List buckets
POST   /_/api/storage/buckets              Create bucket
GET    /_/api/storage/buckets/{id}         Get bucket details
PATCH  /_/api/storage/buckets/{id}         Update bucket
DELETE /_/api/storage/buckets/{id}         Delete bucket

GET    /_/api/storage/objects/{bucket}     List objects (paginated)
POST   /_/api/storage/objects/{bucket}     Upload object (multipart)
GET    /_/api/storage/objects/{bucket}/*   Download object
DELETE /_/api/storage/objects/{bucket}/*   Delete object

GET    /_/api/storage/config               Get storage config
PATCH  /_/api/storage/config               Update storage config
GET    /_/api/storage/stats                Storage statistics
```

---

## Migration to Supabase

When exporting to Supabase, storage data needs special handling:

1. **Schema Export:** Generate `storage.buckets` and `storage.objects` DDL
2. **File Export:**
   - For local: Provide file listing for manual upload to Supabase Storage
   - For S3: Can potentially configure Supabase to use same bucket
3. **RLS Export:** Convert sblite RLS policies to PostgreSQL format

```bash
# Export storage schema
./sblite migrate export --include-storage

# Export file manifest (for manual upload)
./sblite storage export-manifest > files.json
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_STORAGE_BACKEND` | `local` | Storage backend: `local` or `s3` |
| `SBLITE_STORAGE_PATH` | `./storage` | Local storage directory |
| `SBLITE_S3_ENDPOINT` | - | S3 endpoint URL |
| `SBLITE_S3_REGION` | `us-east-1` | S3 region |
| `SBLITE_S3_BUCKET` | - | S3 bucket name |
| `SBLITE_S3_ACCESS_KEY` | - | S3 access key |
| `SBLITE_S3_SECRET_KEY` | - | S3 secret key |
| `SBLITE_S3_FORCE_PATH_STYLE` | `false` | Use path-style S3 URLs |
| `SBLITE_STORAGE_MAX_SIZE` | `52428800` | Max file size (50MB default) |
| `SBLITE_IMAGE_CACHE_SIZE` | `104857600` | Transform cache size (100MB) |

---

## Implementation Phases

### Phase 1: Core Storage (Local Backend)
1. Database schema (buckets, objects, config tables)
2. Backend interface and local filesystem implementation
3. Basic REST API (upload, download, delete, list)
4. Bucket management (create, delete, empty)
5. Integration with existing server setup

**Estimated scope:** ~800-1000 lines of Go

### Phase 2: Access Control
1. RLS integration for storage_objects table
2. Public/private bucket handling
3. Owner-based permissions
4. Signed URL generation and validation

**Estimated scope:** ~400-500 lines of Go

### Phase 3: S3 Backend
1. minio-go integration
2. S3 configuration and connection
3. Backend switching (local <-> S3)
4. CLI commands for S3 configuration

**Estimated scope:** ~300-400 lines of Go

### Phase 4: Image Transformation
1. imaging library integration
2. Transform endpoint (`/render/image/*`)
3. Transformation caching (LRU)
4. WebP auto-conversion

**Estimated scope:** ~400-500 lines of Go

### Phase 5: Dashboard & CLI
1. Dashboard storage UI (buckets, file browser)
2. CLI commands (storage config, buckets, objects)
3. Storage statistics and usage tracking

**Estimated scope:** ~600-800 lines (Go + JS)

### Phase 6: Polish & Migration
1. Migration export support
2. E2E tests with @supabase/supabase-js
3. Documentation updates
4. CLAUDE.md updates

**Estimated scope:** ~300-400 lines + tests

---

## Dependencies to Add

```go
// go.mod additions
require (
    github.com/minio/minio-go/v7 v7.0.x      // S3 client
    github.com/disintegration/imaging v1.6.x // Image processing
)
```

Both are pure Go with no CGO requirements.

---

## Security Considerations

1. **Path Traversal Prevention:**
   - Sanitize all file paths
   - Reject paths with `..`, absolute paths, null bytes
   - Validate path components against allowed characters

2. **MIME Type Validation:**
   - Detect content type from file content, not just extension
   - Validate against bucket's allowed MIME types
   - Use `http.DetectContentType()` for detection

3. **File Size Limits:**
   - Enforce bucket-level and global limits
   - Stream uploads to avoid memory exhaustion
   - Return `413 Payload Too Large` for violations

4. **Signed URL Security:**
   - Use HMAC-SHA256 with JWT secret
   - Include expiration in signed data
   - Validate all claims before serving

5. **RLS Bypass Prevention:**
   - Always check RLS unless bucket is public (read only)
   - Service role should only be used for admin operations
   - Never expose service key to client

---

## Testing Strategy

### Unit Tests
- Backend interface mocking
- Path sanitization
- MIME type detection
- Signed URL generation/validation
- Image transformation

### Integration Tests
- Local backend file operations
- S3 backend (using MinIO in Docker for tests)
- RLS policy evaluation
- Bucket CRUD operations

### E2E Tests
- Full compatibility with `@supabase/supabase-js` storage client
- Upload, download, list, delete workflows
- Signed URL flows
- Public bucket access
- Image transformation URLs

---

## Open Questions

1. **Thumbnail Pregeneration:** Should we generate thumbnails on upload (like Pocketbase) or only on-demand (like Supabase)?
   - **Recommendation:** On-demand with caching (Supabase approach) - simpler, less storage

2. **Resumable Uploads (TUS):** Should we support TUS protocol for large file uploads?
   - **Recommendation:** Phase 2 feature - start with standard multipart uploads

3. **Versioning:** Should we support object versioning?
   - **Recommendation:** Future feature - add version column but don't implement initially

4. **Multitenancy:** How to handle storage paths for multi-tenant setups?
   - **Recommendation:** Use bucket per tenant or prefix within bucket

---

## References

- [Supabase Storage Docs](https://supabase.com/docs/guides/storage)
- [Supabase Storage API Swagger](https://supabase.github.io/storage/)
- [Supabase Storage Source](https://github.com/supabase/storage)
- [Pocketbase Filesystem Docs](https://pocketbase.io/docs/go-filesystem/)
- [minio-go Documentation](https://pkg.go.dev/github.com/minio/minio-go/v7)
- [disintegration/imaging](https://pkg.go.dev/github.com/disintegration/imaging)
