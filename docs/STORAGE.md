# sblite Storage API

sblite provides a Supabase-compatible Storage API for file uploads, downloads, and management. The API is fully compatible with the `@supabase/supabase-js` Storage client.

## Overview

- **100% Supabase API compatible** - Works with the official Supabase JavaScript client
- **Multiple storage backends** - Local filesystem or S3-compatible storage (AWS S3, MinIO, R2, etc.)
- **Public and private buckets** - Support for public URLs and authenticated access
- **Signed URLs** - Time-limited access URLs for downloads and uploads without authentication
- **Row Level Security (RLS)** - Supabase-compatible RLS policies for fine-grained access control
- **File size limits** - Per-bucket configurable size limits
- **MIME type restrictions** - Per-bucket allowed MIME types

## Configuration

### Local Storage (Default)

Files are stored on the local filesystem:

```bash
# Via CLI flag
./sblite serve --storage-path ./my-storage

# Via environment variable
export SBLITE_STORAGE_PATH=./my-storage
./sblite serve

# Default: ./storage
```

### S3-Compatible Storage

For production deployments, you can use S3 or any S3-compatible service (MinIO, Cloudflare R2, Backblaze B2, DigitalOcean Spaces, etc.):

```bash
# AWS S3
./sblite serve \
  --storage-backend=s3 \
  --s3-bucket=my-bucket \
  --s3-region=us-east-1 \
  --s3-access-key=AKIAIOSFODNN7EXAMPLE \
  --s3-secret-key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# MinIO (self-hosted S3-compatible)
./sblite serve \
  --storage-backend=s3 \
  --s3-endpoint=http://localhost:9000 \
  --s3-bucket=my-bucket \
  --s3-access-key=minioadmin \
  --s3-secret-key=minioadmin \
  --s3-path-style=true

# Cloudflare R2
./sblite serve \
  --storage-backend=s3 \
  --s3-endpoint=https://ACCOUNT_ID.r2.cloudflarestorage.com \
  --s3-bucket=my-bucket \
  --s3-access-key=your-access-key \
  --s3-secret-key=your-secret-key
```

#### S3 Configuration Options

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--storage-backend` | `SBLITE_STORAGE_BACKEND` | Backend type: `local` or `s3` (default: `local`) |
| `--storage-path` | `SBLITE_STORAGE_PATH` | Local storage directory (default: `./storage`) |
| `--s3-endpoint` | `SBLITE_S3_ENDPOINT` | S3 endpoint URL (optional for AWS, required for others) |
| `--s3-region` | `SBLITE_S3_REGION` | AWS region (e.g., `us-east-1`) |
| `--s3-bucket` | `SBLITE_S3_BUCKET` | S3 bucket name (required for S3 backend) |
| `--s3-access-key` | `SBLITE_S3_ACCESS_KEY` | S3 access key ID |
| `--s3-secret-key` | `SBLITE_S3_SECRET_KEY` | S3 secret access key |
| `--s3-path-style` | `SBLITE_S3_PATH_STYLE` | Use path-style addressing (required for MinIO) |

#### IAM Roles (AWS)

On AWS EC2/ECS/Lambda, you can use IAM roles instead of access keys:

```bash
./sblite serve \
  --storage-backend=s3 \
  --s3-bucket=my-bucket \
  --s3-region=us-east-1
# Credentials are automatically loaded from IAM role
```

## API Endpoints

All endpoints are mounted at `/storage/v1`.

### Bucket Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/storage/v1/bucket` | GET | List all buckets |
| `/storage/v1/bucket` | POST | Create a new bucket |
| `/storage/v1/bucket/{id}` | GET | Get bucket details |
| `/storage/v1/bucket/{id}` | PUT | Update bucket settings |
| `/storage/v1/bucket/{id}` | DELETE | Delete an empty bucket |
| `/storage/v1/bucket/{id}/empty` | POST | Remove all objects from bucket |

### Object Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/storage/v1/object/list/{bucket}` | POST | List objects in bucket |
| `/storage/v1/object/{bucket}/*` | POST | Upload a file |
| `/storage/v1/object/{bucket}/*` | PUT | Upload/update a file |
| `/storage/v1/object/{bucket}/*` | GET | Download a file |
| `/storage/v1/object/{bucket}/*` | DELETE | Delete a file |
| `/storage/v1/object/{bucket}` | DELETE | Batch delete files |
| `/storage/v1/object/public/{bucket}/*` | GET | Download from public bucket (no auth) |
| `/storage/v1/object/copy` | POST | Copy a file |
| `/storage/v1/object/move` | POST | Move/rename a file |

### Signed URL Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/storage/v1/object/sign/{bucket}/*` | POST | Create signed download URL |
| `/storage/v1/object/sign/{bucket}` | POST | Batch create signed download URLs |
| `/storage/v1/object/sign/{bucket}/*` | GET | Download via signed URL (no auth) |
| `/storage/v1/object/upload/sign/{bucket}/*` | POST | Create signed upload URL |
| `/storage/v1/object/upload/sign/{bucket}/*` | PUT | Upload via signed URL (no auth) |

## Usage with @supabase/supabase-js

### Creating a Bucket

```typescript
const { data, error } = await supabase.storage.createBucket('avatars', {
  public: true,
  fileSizeLimit: 1024 * 1024, // 1MB
  allowedMimeTypes: ['image/png', 'image/jpeg', 'image/gif']
})
```

### Uploading Files

```typescript
// Upload a file
const file = new File(['Hello World'], 'hello.txt', { type: 'text/plain' })
const { data, error } = await supabase.storage
  .from('documents')
  .upload('folder/hello.txt', file)

// Upload with upsert (overwrite if exists)
const { data, error } = await supabase.storage
  .from('documents')
  .upload('folder/hello.txt', file, { upsert: true })
```

### Downloading Files

```typescript
// Download a file
const { data, error } = await supabase.storage
  .from('documents')
  .download('folder/hello.txt')

// data is a Blob
const text = await data.text()
```

### Listing Files

```typescript
// List all files in bucket root
const { data, error } = await supabase.storage
  .from('documents')
  .list()

// List files in a folder
const { data, error } = await supabase.storage
  .from('documents')
  .list('folder')

// List with options
const { data, error } = await supabase.storage
  .from('documents')
  .list('', {
    limit: 100,
    offset: 0,
    search: 'hello',
    sortBy: { column: 'name', order: 'asc' }
  })
```

### Deleting Files

```typescript
// Delete a single file
const { data, error } = await supabase.storage
  .from('documents')
  .remove(['folder/hello.txt'])

// Delete multiple files
const { data, error } = await supabase.storage
  .from('documents')
  .remove(['file1.txt', 'file2.txt', 'folder/file3.txt'])
```

### Moving and Copying Files

```typescript
// Move a file
const { data, error } = await supabase.storage
  .from('documents')
  .move('old-path.txt', 'new-path.txt')

// Copy a file
const { data, error } = await supabase.storage
  .from('documents')
  .copy('original.txt', 'copy.txt')
```

### Public URLs

```typescript
// Get public URL for a file in a public bucket
const { data } = supabase.storage
  .from('public-bucket')
  .getPublicUrl('image.png')

console.log(data.publicUrl)
// http://localhost:8080/storage/v1/object/public/public-bucket/image.png

// With download parameter
const { data } = supabase.storage
  .from('public-bucket')
  .getPublicUrl('image.png', { download: 'custom-filename.png' })
```

### Signed URLs

Signed URLs provide time-limited access to private files without requiring authentication. RLS policies are checked at URL creation time, not access time.

#### Creating a Signed Download URL

```typescript
// Create a signed URL that expires in 60 seconds
const { data, error } = await supabase.storage
  .from('private-bucket')
  .createSignedUrl('folder/document.pdf', 60)

console.log(data.signedUrl)
// http://localhost:8080/storage/v1/object/sign/private-bucket/folder/document.pdf?token=eyJ...

// With download option (sets Content-Disposition header)
const { data, error } = await supabase.storage
  .from('private-bucket')
  .createSignedUrl('document.pdf', 60, { download: 'custom-name.pdf' })
```

#### Batch Creating Signed URLs

```typescript
// Create multiple signed URLs at once
const { data, error } = await supabase.storage
  .from('private-bucket')
  .createSignedUrls(['file1.txt', 'file2.txt', 'folder/file3.txt'], 60)

// Returns array of results
data.forEach(item => {
  console.log(item.path, item.signedUrl, item.error)
})
```

#### Creating a Signed Upload URL

```typescript
// Create a signed URL for uploading (default 2 hour expiry)
const { data, error } = await supabase.storage
  .from('uploads')
  .createSignedUploadUrl('user-uploads/new-file.txt')

// Upload using the signed URL
const { data: uploadData, error: uploadError } = await supabase.storage
  .from('uploads')
  .uploadToSignedUrl('user-uploads/new-file.txt', data.token, file)
```

#### Security Considerations

- **RLS at creation**: Access control is enforced when the signed URL is created, not when it's used
- **Token expiry**: Download URLs use the specified expiry (seconds). Upload URLs default to 2 hours.
- **Path binding**: Tokens are bound to specific bucket/path combinations and cannot be reused for other files
- **Token validation**: Invalid or expired tokens return 401 Unauthorized; wrong path returns 403 Forbidden

### Bucket Management

```typescript
// List all buckets
const { data, error } = await supabase.storage.listBuckets()

// Get bucket details
const { data, error } = await supabase.storage.getBucket('avatars')

// Update bucket
const { data, error } = await supabase.storage.updateBucket('avatars', {
  public: true,
  fileSizeLimit: 2 * 1024 * 1024
})

// Empty bucket
const { data, error } = await supabase.storage.emptyBucket('avatars')

// Delete bucket (must be empty)
const { data, error } = await supabase.storage.deleteBucket('avatars')
```

## Database Schema

The storage system uses two tables:

### storage_buckets

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key (same as name) |
| name | TEXT | Unique bucket name |
| owner | TEXT | Owner identifier |
| owner_id | TEXT | Owner user ID |
| public | INTEGER | 1 if public, 0 if private |
| file_size_limit | INTEGER | Max file size in bytes |
| allowed_mime_types | TEXT | JSON array of allowed types |
| created_at | TEXT | ISO 8601 timestamp |
| updated_at | TEXT | ISO 8601 timestamp |

### storage_objects

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key (UUID) |
| bucket_id | TEXT | References storage_buckets(id) |
| name | TEXT | Object path/name |
| owner | TEXT | Owner identifier |
| owner_id | TEXT | Owner user ID |
| metadata | TEXT | JSON metadata |
| path_tokens | TEXT | JSON array of path components |
| user_metadata | TEXT | User-defined JSON metadata |
| version | TEXT | Object version |
| size | INTEGER | File size in bytes |
| mime_type | TEXT | MIME type |
| etag | TEXT | ETag for caching |
| last_accessed_at | TEXT | Last access timestamp |
| created_at | TEXT | ISO 8601 timestamp |
| updated_at | TEXT | ISO 8601 timestamp |

## File Storage

Files are stored on the local filesystem in the configured storage directory:

```
storage/
├── bucket-name/
│   ├── file1.txt
│   ├── folder/
│   │   └── file2.txt
│   └── another-folder/
│       └── nested/
│           └── file3.txt
```

Each bucket has its own subdirectory. Object paths within the bucket map directly to file paths.

## Row Level Security (RLS)

sblite supports Supabase-compatible RLS policies for storage. Policies are applied to the `storage_objects` table to control file access.

### Default Behavior

- **RLS is enabled by default** on the `storage_objects` table
- Without any policies, authenticated users cannot access files
- Service role API key bypasses RLS completely
- Public bucket endpoint (`/object/public/*`) is unaffected by RLS

### Storage Helper Functions

Use these functions in RLS policy expressions:

| Function | Description | Example |
|----------|-------------|---------|
| `storage.filename(name)` | Extract filename from path | `'folder/image.png'` → `'image.png'` |
| `storage.foldername(name)` | Extract folder path | `'folder/sub/file.txt'` → `'folder/sub'` |
| `storage.extension(name)` | Extract file extension | `'image.png'` → `'png'` |

### Common Policy Patterns

#### Owner-based access (users can only access their own files):

```sql
-- SELECT policy (download)
CREATE POLICY "Users can view own files" ON storage_objects
FOR SELECT USING (owner_id = auth.uid());

-- INSERT policy (upload)
CREATE POLICY "Users can upload own files" ON storage_objects
FOR INSERT WITH CHECK (owner_id = auth.uid());

-- DELETE policy
CREATE POLICY "Users can delete own files" ON storage_objects
FOR DELETE USING (owner_id = auth.uid());
```

#### Bucket-specific access:

```sql
CREATE POLICY "Allow public bucket access" ON storage_objects
FOR SELECT USING (bucket_id = 'public-assets');
```

#### File type restrictions:

```sql
CREATE POLICY "Only images allowed" ON storage_objects
FOR INSERT WITH CHECK (storage.extension(name) IN ('png', 'jpg', 'gif'));
```

#### Folder-based access:

```sql
CREATE POLICY "Users can access their folder" ON storage_objects
FOR SELECT USING (storage.foldername(name) = auth.uid());
```

### Storage Operation Mapping

| Storage Operation | RLS Command | Description |
|-------------------|-------------|-------------|
| Upload (POST/PUT) | INSERT | `WITH CHECK` expression evaluated |
| Download (GET) | SELECT | `USING` expression evaluated |
| Delete (DELETE) | DELETE | `USING` expression evaluated |
| List (POST /list) | SELECT | `USING` expression evaluated |
| Copy | SELECT + INSERT | Both checked |
| Move | SELECT + DELETE + INSERT | All three checked |

### Creating Policies

Use the Dashboard API to create storage RLS policies:

```bash
curl -X POST http://localhost:8080/_/api/policies \
  -H "Content-Type: application/json" \
  -H "apikey: YOUR_SERVICE_ROLE_KEY" \
  -d '{
    "table_name": "storage_objects",
    "policy_name": "owner_access",
    "command": "SELECT",
    "using_expr": "owner_id = auth.uid()"
  }'
```

## Limitations

Current implementation limitations compared to Supabase:

1. **No image transformations** - Image resizing/transformations not supported

## Error Responses

All errors follow the Supabase error format:

```json
{
  "statusCode": 404,
  "error": "not_found",
  "message": "Object not found"
}
```

Common error codes:

| Code | Status | Description |
|------|--------|-------------|
| `not_found` | 404 | Bucket or object not found |
| `already_exists` | 409 | Bucket or object already exists |
| `bucket_not_empty` | 400 | Cannot delete non-empty bucket |
| `not_public` | 400 | Bucket is not public |
| `invalid_request` | 400 | Invalid request body or parameters |
| `file_too_large` | 413 | File exceeds bucket size limit |
| `invalid_mime_type` | 400 | MIME type not in allowed list |
| `access_denied` | 403 | Access denied by RLS policy |
